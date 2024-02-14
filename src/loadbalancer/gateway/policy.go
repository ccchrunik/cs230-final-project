package gateway

import (
	"log"
	"math/rand"
	"net/http"
)

type Policy interface {
	Select(*ServerPool) *Backend
}

type RoundRobinPolicy struct {
	current int
}

func (rr *RoundRobinPolicy) Select(p *ServerPool) *Backend {
	backend := p.Get(rr.current % p.Len())
	rr.current = (rr.current + 1) % p.Len()
	return backend
}

type RandomPolicy struct {
}

func (rr *RandomPolicy) Select(p *ServerPool) *Backend {
	return p.alives[rand.Intn(len(p.alives))]
}

func loadbalancing(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	next := gtw.NextServer()
	if next != nil {
		next.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

// type LeastConnectionPolicy struct{}
