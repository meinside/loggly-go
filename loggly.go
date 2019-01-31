package loggly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// Constants
const (
	numQueue = 32

	bulkEndpointURLFormat  = "https://logs-01.loggly.com/bulk/%s/tag/bulk/"
	bulkRequestContentType = "text/plain"

	keyTimestamp = "timestamp"

	JSONTimestampFormat = "2006-01-02T15:04:05.999Z"
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
		endpointURL: fmt.Sprintf(bulkEndpointURLFormat, token),
		client: &http.Client{
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 300 * time.Second,
				}).Dial,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		channel: make(chan interface{}, numQueue),
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
func (l *Loggly) LogSync(obj interface{}) error {
	return l.send(obj)
}

// Stop stops logger
func (l *Loggly) Stop() {
	l.stop <- struct{}{}
}

func (l *Loggly) send(obj interface{}) (err error) {
	var data []byte
	if data, err = json.Marshal(obj); err == nil {
		l.Lock()

		var req *http.Request
		if req, err = http.NewRequest("POST", l.endpointURL, bytes.NewBuffer(data)); err == nil {
			req.Header.Set("Content-Type", bulkRequestContentType)

			var resp *http.Response
			resp, err = l.client.Do(req)

			if resp != nil {
				defer resp.Body.Close()
			}

			if err == nil {
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

	return err
}

// Timestamp generates key and value for current time's timestamp (in ISO-8601)
//
// https://www.loggly.com/docs/automated-parsing/#json
func Timestamp() (key, value string) {
	return keyTimestamp, time.Now().UTC().Format(JSONTimestampFormat)
}
