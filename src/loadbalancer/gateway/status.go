package gateway

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Context key
const (
	Attempts int = iota
	Retry
)

func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 1
}

func GetRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("Site unreachable, error: ", err)
		return false
	}
	defer conn.Close()
	return true
}

func healthCheck() {
	t := time.NewTicker(time.Minute)
	for {
		select {
		case <-t.C:
			log.Println("Starting health check...")
			gtw.healthCheck()
			log.Println("Health check completed")
		}
	}
}
