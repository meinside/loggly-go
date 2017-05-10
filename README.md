# loggly-go

A Go library for [Loggly](https://www.loggly.com/).

## How to get it

```bash
$ go get github.com/meinside/loggly-go
```

## How to use it

```go
// sample.go
package main

import (
	"time"

	"github.com/meinside/loggly-go"
)

func main() {
	logger := loggly.New("XXXXXX-0000-YYYY-ZZZZ-0000000000000")

	// can log any type of variable
	logger.Log("Hello loggly.")	// a string,
	for i := 0; i < 42; i++ {
		logger.Log(i)	// a number,
	}
	logger.Log(struct {
		Severity string `json:"severity"`
		Message  string `json:"message"`
	}{
		Severity: "WARN",
		Message:  "This is a warning.",
	})	// or even a struct

	// XXX - wait until all logs are transfered
	time.Sleep(40 * time.Second)
}
```

## License

MIT

