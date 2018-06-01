package loggly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// Constants
const (
	EndpointURLFormat  = "http://logs-01.loggly.com/bulk/%s/tag/bulk/"
	RequestContentType = "text/plain"
	NumQueue           = 32
)

// Loggly struct
type Loggly struct {
	EndpointURL string
	client      *http.Client
	channel     chan interface{}

	sync.Mutex
}

// New gets a new logger
func New(token string) *Loggly {
	logger := Loggly{
		EndpointURL: fmt.Sprintf(EndpointURLFormat, token),
		client:      &http.Client{},
		channel:     make(chan interface{}, NumQueue),
	}

	// monitor incoming objects
	go func() {
		for o := range logger.channel {
			logger.send(o)
		}
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

func (l *Loggly) send(obj interface{}) {
	var err error

	var data []byte
	if data, err = json.Marshal(obj); err == nil {
		l.Lock()

		var req *http.Request
		if req, err = http.NewRequest("POST", l.EndpointURL, bytes.NewBuffer(data)); err == nil {
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
