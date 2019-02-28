package loggly

// Loggly logger library which sends logs synchronously or asynchronously

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

// Constants
const (
	numQueue          = 32
	retryDelaySeconds = 3
	retryCountLimit   = 3

	bulkEndpointURLFormat  = "https://logs-01.loggly.com/bulk/%s/tag/bulk/"
	bulkRequestContentType = "text/plain"

	keyTimestamp = "timestamp"

	JSONTimestampFormat = "2006-01-02T15:04:05.999Z"
)

// Loggly struct
type Loggly struct {
	endpointURL string

	client *http.Client

	// for async sender loop
	request chan logRequest
	failed  chan logRequest
	stop    chan struct{}
	running bool
}

type logRequest struct {
	retried int
	data    interface{}
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
		request: make(chan logRequest, numQueue),
		failed:  make(chan logRequest, numQueue),
		stop:    make(chan struct{}),
	}

	// monitor incoming objects
	go func() {
		log.Printf("loggly logger starting async sender loop...")

		logger.running = true

	loop:
		for {
			select {
			case o := <-logger.request:
				go func(request logRequest) {
					if err := logger.send(request.data); err != nil {
						// retry on error
						logger.failed <- logRequest{retried: 0, data: request.data}
					}
				}(o)
			case f := <-logger.failed:
				go func(request logRequest) {
					if request.retried >= retryCountLimit {
						log.Printf("loggly logger dropping request with too many retries: %d", retryCountLimit)
					} else {
						time.Sleep(retryDelaySeconds * time.Second)

						log.Printf("loggly logger resending failed request...")

						logger.request <- logRequest{retried: request.retried + 1, data: request.data}
					}
				}(f)
			case <-logger.stop:
				break loop
			}
		}

		logger.running = false

		log.Printf("loggly logger stopped async sender loop")
	}()

	return &logger
}

// Log logs given object asynchronously
func (l *Loggly) Log(obj interface{}) {
	if l.running {
		l.request <- logRequest{retried: 0, data: obj}
	} else {
		log.Printf("loggly logger async sender loop is not running")
	}
}

// LogSync logs given object synchronously
func (l *Loggly) LogSync(obj interface{}) error {
	return l.send(obj)
}

// Stop stops logger's async sender loop
func (l *Loggly) Stop() {
	log.Printf("loggly logger stopping async sender loop...")

	l.stop <- struct{}{}
}

func (l *Loggly) send(obj interface{}) (err error) {
	var data []byte
	if data, err = json.Marshal(obj); err == nil {
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
					var body []byte
					if resp.Body != nil {
						body, err = ioutil.ReadAll(resp.Body)
					}

					if body != nil {
						log.Printf("loggly returned http status %d: %s", resp.StatusCode, string(body))
					} else {
						log.Printf("loggly returned http status %d", resp.StatusCode)
					}
				}
			}
		}
	}

	if err != nil {
		log.Printf("loggly error: %s", err)
	}

	return err
}

// Timestamp generates key and value for current time's timestamp (in ISO-8601)
//
// https://www.loggly.com/docs/automated-parsing/#json
func Timestamp() (key, value string) {
	return keyTimestamp, time.Now().UTC().Format(JSONTimestampFormat)
}
