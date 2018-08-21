package loggly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Constants
const (
	EndpointURLFormat  = "https://logs-01.loggly.com/bulk/%s/tag/bulk/"
	RequestContentType = "text/plain"
	NumQueue           = 32
	TimeoutSeconds     = 5
)

// Loggly struct
type Loggly struct {
	endpointURL string
	client      *http.Client
	channel     chan interface{}
	stop        chan struct{}

	sync.Mutex
}

// New gets a new logger
func New(token string) *Loggly {
	logger := Loggly{
		endpointURL: fmt.Sprintf(EndpointURLFormat, token),
		client: &http.Client{
			Timeout: TimeoutSeconds * time.Second,
		},
		channel: make(chan interface{}, NumQueue),
		stop:    make(chan struct{}),
	}

	// monitor incoming objects
	go func() {
		log.Println("Starting logger...")

		for {
			select {
			case o := <-logger.channel:
				logger.send(o)
			case <-logger.stop:
				break
			}
		}

		log.Println("Stopping logger...")
	}()

	return &logger
}

// Log logs given object asynchronously
func (l *Loggly) Log(obj interface{}) {
	go func() {
		l.channel <- obj
	}()
}

// LogSync logs given object synchronously
func (l *Loggly) LogSync(obj interface{}) {
	l.send(obj)
}

// Stop stops logger
func (l *Loggly) Stop() {
	l.stop <- struct{}{}
}

func (l *Loggly) send(obj interface{}) {
	var err error

	var data []byte
	if data, err = json.Marshal(obj); err == nil {
		l.Lock()

		var req *http.Request
		if req, err = http.NewRequest("POST", l.endpointURL, bytes.NewBuffer(data)); err == nil {
			req.Header.Set("Content-Type", RequestContentType)

			var resp *http.Response
			if resp, err = l.client.Do(req); err == nil {
				if resp.StatusCode != 200 {
					log.Printf("Loggly: Returned HTTP status %d", resp.StatusCode)
				}
			}
		}

		l.Unlock()
	}

	if err != nil {
		log.Printf("Loggly Error: %s", err)
	}
}
