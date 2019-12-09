package main

import (
	"encoding/json"
	"flag"
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

var correct []string
var correctLock sync.RWMutex

var jobs map[string]*Job
var jobsLock sync.RWMutex

var jobsCompleted []*Job
var jobsCompletedLock sync.RWMutex

var workers map[string]*Worker
var workerLock sync.RWMutex

func main() {
	addr := flag.String("addr", ":8080", "http service address")
	flag.Parse()

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
	go SaveResults();

	jobsLock.Lock()
	jobs = make(map[string]*Job)
	jobsLock.Unlock()

	// Load remaining links
	ldata, err := ioutil.ReadFile("permutations.txt")
	if err != nil {
		log.Fatalln(err)
	}
	for _, link := range strings.Split(string(ldata), "\n") {
		trimmed := strings.TrimSpace(link)
		if len(trimmed) > 0 {
			found := false
			jobsCompletedLock.Lock()
			for _, c := range jobsCompleted {
				if c.URL.String() == trimmed {
					found = true
				}
			}
			jobsCompletedLock.Unlock()

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

	http.HandleFunc("/", home)
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

func SaveResults(){
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
		jobsCompleted = append(jobsCompleted, &result)
		jobsCompletedLock.Unlock()

		if strings.Contains(result.URL.Host, "bit.do"){

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
			for i := range jobs {
				if jobs[i].Worker == "" {
					jobs[i].Worker = s.Hostname
					s.Current = jobs[i]
					s.Send <- *jobs[i]
					break
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
