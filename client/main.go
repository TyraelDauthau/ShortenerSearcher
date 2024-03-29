package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

type Job struct {
	URL        *url.URL `json:"url"`
	StatusCode int      `json:"status"`
	Body       string   `json:"body"`
	Correct    bool     `json:"correct"`
}

var clientPort int
var clientDevice, clientHost, clientConfig string

func main() {
	flag.StringVar(&clientDevice, "hostname", "", "Current hostname for client")
	flag.StringVar(&clientHost, "host", "127.0.0.1", "Device Server Hostname")
	flag.IntVar(&clientPort, "port", 8080, "Device Server Port")
	flag.Parse()

	eHostname := os.Getenv("CLIENT_HOSTNAME")
	if clientDevice == "" && eHostname != "" {
		clientDevice = eHostname
	}

	eHost := os.Getenv("CLIENT_HOST")
	if clientHost == "127.0.0.1" && eHost != "127.0.0.1" {
		clientHost = eHost
	}

	ePort, err := strconv.Atoi(os.Getenv("CLIENT_PORT"))
	if err == nil && clientPort == 8080 && ePort != 8080 {
		clientPort = ePort
	}

	// Setup Communication
	u, err := url.Parse(fmt.Sprintf("ws://%s:%d/?hostname=%s", clientHost, clientPort, clientDevice))
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Connecting to", u.String())

	dialer := websocket.Dialer{}
	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	/*c.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.SetPongHandler(func(string) error { c.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })*/

	Send := make(chan Job)
	go func(con *websocket.Conn, send chan Job) {
		ticker := time.NewTicker(50 * time.Second)
		defer func() {
			ticker.Stop()
			con.Close()
		}()

		for {
			select {
			case data := <-send:
				con.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := con.WriteJSON(&data); err != nil {
					log.Println(err)
					return
				}
			case <-ticker.C:
				con.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := con.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					return
				}
			}
		}
	}(c, Send)

	client := &http.Client{}

	dialSocksProxy, err := proxy.SOCKS5("unix", "/var/run/tor/socks", nil, proxy.Direct)
	if err == nil {
		log.Println("Using TOR proxy")
		client.Transport = &http.Transport{Dial: dialSocksProxy.Dial}
	}

	for {
		// Parse Job
		var job Job
		if err := c.ReadJSON(&job); err != nil {
			log.Println(err)
			break
		}

		// Handle Job
		for retry := 0; retry < 3; retry++ {
			req, err := http.NewRequest("GET", job.URL.String(), nil)
			if err != nil {
				continue
			}

			res, err := client.Do(req)
			if err != nil {
				continue
			}

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				continue
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
				time.Sleep(30 * time.Second)
				continue
			}

			job.StatusCode = res.StatusCode
			job.Body = string(body)

			// Test if link was valid
			if strings.Contains(job.URL.Host, "bit.do") {
				if job.StatusCode == http.StatusOK && !strings.Contains(job.Body, "404 Not Found") {
					job.Correct = true
				}
			} else {
				if job.StatusCode == http.StatusOK {
					job.Correct = true
				}
			}

			break
		}

		log.Println("Completed:", job.URL.String(), job.StatusCode)

		// Send Result
		Send <- job

		time.Sleep(10 * time.Millisecond)
	}
}
