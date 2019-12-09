package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var correct = []string{}
var correctLock sync.RWMutex

var jobs map[string]*Job
var jobsLock sync.RWMutex

var jobsCompleted = []string{}
var jobsCompletedLock sync.RWMutex

var workers map[string]*Worker
var workerLock sync.RWMutex

func main() {
	addr := flag.String("addr", ":8080", "http service address")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)

	workers = make(map[string]*Worker)

	// Load completed links
	fComplete, err := os.Open("completed.json")
	if err == nil {
		jobsCompletedLock.Lock()
		if err := json.NewDecoder(fComplete).Decode(&jobsCompleted); err != nil {
			log.Fatalln(err)
		}
		jobsCompletedLock.Unlock()
	}
	fComplete.Close()
	go SaveResults()

	jobsLock.Lock()
	jobs = make(map[string]*Job)
	jobsLock.Unlock()

	// Load remaining links
	ldata, err := ioutil.ReadFile("permutations.txt")
	if err != nil {
		for _, link := range strings.Split(string(ldata), "\n") {
			trimmed := strings.TrimSpace(link)
			if len(trimmed) > 0 {
				found := false
				jobsCompletedLock.RLock()
				for _, c := range jobsCompleted {
					if c == trimmed {
						found = true
					}
				}
				jobsCompletedLock.RUnlock()

				if !found {
					u, err := url.Parse(trimmed)
					if err != nil {
						log.Fatalln(err)
					}

					jobsLock.Lock()
					jobs[u.String()] = &Job{URL: u, StatusCode: -1, Body: ""}
					jobsLock.Unlock()
				}
			}
		}
	}
	go SavePermutations()

	http.HandleFunc("/", home)
	http.HandleFunc("/permutations", permutationList)
	http.HandleFunc("/permutations/count", permutationCount)
	http.HandleFunc("/permutations/remaining", permutationRemainingList)
	http.HandleFunc("/permutations/remaining/count", permutationRemainingCount)
	http.HandleFunc("/permutations/complete", permutationCompletedList)
	http.HandleFunc("/permutations/complete/count", permutationCompletedCount)
	http.HandleFunc("/permutations/succeeded", succeededList)
	http.HandleFunc("/permutations/succeeded/count", succeededCount)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

var upgrader = websocket.Upgrader{
	EnableCompression: true,
}

func home(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	// Setup device connection
	worker := &Worker{Connection: c, Hostname: r.FormValue("hostname"), Current: nil, Send: make(chan Job, 10), AwaitingResponse: false}
	workerLock.Lock()
	workers[worker.Hostname] = worker
	workerLock.Unlock()
	log.Println("Added", worker.Hostname)

	// Start write loop
	go worker.Write()

	// Setup app loop
	go worker.Run()

	// Setup read loop
	worker.Read()

	// Socket closed, remove device
	workerLock.Lock()
	delete(workers, worker.Hostname)
	workerLock.Unlock()

	// Clear any jobs that the worker was acting on
	for i := range jobs {
		if jobs[i].Worker == worker.Hostname {
			jobs[i].Worker = ""
		}
	}

	log.Println("Deleted", worker.Hostname)
}

func permutationList(w http.ResponseWriter, r *http.Request) {
	// List of urls addition
	if r.Method == "POST" {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		var Buf bytes.Buffer
		io.Copy(&Buf, file)

		var newJobs []*Job

		contents := Buf.String()
		for _, link := range strings.Split(contents, "\n") {
			trimmed := strings.TrimSpace(link)
			if len(trimmed) > 0 {
				found := false
				jobsCompletedLock.RLock()
				for _, c := range jobsCompleted {
					if c == trimmed {
						found = true
					}
				}
				jobsCompletedLock.RUnlock()

				if !found {
					u, err := url.Parse(trimmed)
					if err != nil {
						log.Fatalln(err)
					}
					
					newJobs = append(newJobs, &Job{URL: u, StatusCode: -1, Body: ""})
				}
			}
		}

		jobsLock.Lock()
		for _, j := range newJobs {
			jobs[j.URL.String()] = j
		}
		jobsLock.Unlock()
		return
	}

	// Single url addition
	input := r.FormValue("url")
	if input != "" {
		// Check if we have already completed it
		jobsCompletedLock.RLock()
		for _, job := range jobsCompleted {
			if input == job {
				w.WriteHeader(http.StatusAlreadyReported)
				return
			}
		}
		jobsCompletedLock.RUnlock()

		// Check if we already have it as a job
		jobsLock.RLock()
		for _, job := range jobs {
			if input == job.URL.String() {
				w.WriteHeader(http.StatusAlreadyReported)
				return
			}
		}
		jobsLock.RUnlock()

		// Parse URL
		u, err := url.Parse(input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add to job list
		jobsLock.Lock()
		jobs[input] = &Job{URL: u, StatusCode: -1, Body: ""}
		jobsLock.Unlock()

		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Return list
	result := []string{}

	jobsLock.RLock()
	for _, job := range jobs {
		result = append(result, job.URL.String())
	}
	jobsLock.RUnlock()

	jobsCompletedLock.RLock()
	for _, job := range jobsCompleted {
		result = append(result, job)
	}
	jobsCompletedLock.RUnlock()

	js, err := json.Marshal(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func permutationCount(w http.ResponseWriter, r *http.Request) {
	result := 0

	jobsLock.RLock()
	result += len(jobs)
	jobsLock.RUnlock()

	jobsCompletedLock.RLock()
	result += len(jobsCompleted)
	jobsCompletedLock.RUnlock()

	js, err := json.Marshal(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func permutationRemainingList(w http.ResponseWriter, r *http.Request) {
	result := []string{}

	jobsLock.RLock()
	for _, link := range jobs {
		result = append(result, link.URL.String())
	}
	jobsLock.RUnlock()

	js, err := json.Marshal(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}


func permutationRemainingCount(w http.ResponseWriter, r *http.Request) {
	jobsLock.RLock()
	result := len(jobs)
	jobsLock.RUnlock()

	js, err := json.Marshal(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func permutationCompletedList(w http.ResponseWriter, r *http.Request) {
	jobsCompletedLock.RLock()
	js, err := json.Marshal(&jobsCompleted)
	jobsCompletedLock.RUnlock()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func permutationCompletedCount(w http.ResponseWriter, r *http.Request) {
	jobsCompletedLock.RLock()
	result := len(jobsCompleted)
	jobsCompletedLock.RUnlock()

	js, err := json.Marshal(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func succeededList(w http.ResponseWriter, r *http.Request) {
	correctLock.RLock()
	js, err := json.Marshal(&correct)
	correctLock.RUnlock()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func succeededCount(w http.ResponseWriter, r *http.Request) {
	jobsCompletedLock.RLock()
	result := len(correct)
	jobsCompletedLock.RUnlock()

	js, err := json.Marshal(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func SaveResults() {
	saved := 0

	for {
		jobsCompletedLock.RLock()
		completed := len(jobsCompleted)
		jobsCompletedLock.RUnlock()

		if completed > saved {
			fOut, err := os.Create("complete.json")
			if err != nil {
				log.Fatalln(err)
			}
			jobsCompletedLock.RLock()
			if err := json.NewEncoder(fOut).Encode(&jobsCompleted); err != nil {
				log.Fatalln(err)
			}
			jobsCompletedLock.RUnlock()
			fOut.Close()

			saved = completed
		}
		time.Sleep(1 * time.Second)
	}
}

func SavePermutations() {
	saved := 0

	for {
		result := []string{}

		jobsLock.RLock()
		for _, job := range jobs {
			result = append(result, job.URL.String())
		}
		jobsLock.RUnlock()

		jobsCompletedLock.RLock()
		for _, job := range jobsCompleted {
			result = append(result, job)
		}
		jobsCompletedLock.RUnlock()

		if len(result) > saved {
			fOut, err := os.Create("permutations.txt")
			if err != nil {
				log.Fatalln(err)
			}
			for _, perm := range result {
				fOut.WriteString(perm)

			}
			fOut.Close()

			saved = len(result)
		}
		time.Sleep(1 * time.Second)
	}
}

type Job struct {
	URL        *url.URL `json:"url"`
	StatusCode int      `json:"status"`
	Body       string   `json:"body"`
	Worker     string
}

type Worker struct {
	Connection       *websocket.Conn
	Hostname         string
	Current          *Job
	Send             chan Job
	AwaitingResponse bool
}

func (s *Worker) Read() {
	defer func() {
		s.Connection.Close()
	}()

	s.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	s.Connection.SetPongHandler(func(string) error {
		s.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var result Job
		if err := s.Connection.ReadJSON(&result); err != nil {
			log.Println(err)
			break
		}

		log.Println("Recieved["+s.Hostname+"]", result.URL)
		s.AwaitingResponse = false

		s.Current = nil

		jobsCompletedLock.Lock()
		jobsCompleted = append(jobsCompleted, result.URL.String())
		jobsCompletedLock.Unlock()

		// Test if link was valid
		if strings.Contains(result.URL.Host, "bit.do") {
			if result.StatusCode == http.StatusOK && !strings.Contains(result.Body, "404 Not Found") {
				correctLock.Lock()
				correct = append(correct, result.URL.String())
				correctLock.Unlock()
			}
		} else {
			if result.StatusCode == http.StatusOK {
				correctLock.Lock()
				correct = append(correct, result.URL.String())
				correctLock.Unlock()
			}
		}

		jobsLock.Lock()
		delete(jobs, result.URL.String())
		jobsLock.Unlock()
	}
}

func (s *Worker) Write() {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		s.Connection.Close()
	}()

	for {
		select {
		case data := <-s.Send:
			if err := s.Connection.WriteJSON(&data); err != nil {
				log.Println(err)
				return
			}
		case <-ticker.C:
			if err := s.Connection.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (s *Worker) Run() {
	for {
		// Start job if it has none
		if s.Current == nil {
			jobsLock.Lock()
			for i := range jobs {
				if jobs[i].Worker == "" {
					jobs[i].Worker = s.Hostname
					s.Current = jobs[i]
					s.Send <- *jobs[i]
					break
				}
			}
			jobsLock.Unlock()
		}
		time.Sleep(100 * time.Millisecond)
	}
}
