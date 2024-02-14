package gateway

import (
	"net/http/httputil"
	"net/url"
	"time"
)

func NewBackend(rawUrl string) *Backend {
	serverUrl, err := url.Parse(rawUrl)
	if err != nil {
		return nil
	}

	return &Backend{
		URL:          serverUrl,
		ReverseProxy: httputil.NewSingleHostReverseProxy(serverUrl),
	}
}

type Backend struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy
}

func (b *Backend) rawUrl() string {
	return b.URL.String()
}

func NewServerPool() *ServerPool {
	return &ServerPool{
		alives: []*Backend{},
		dead:   []*Backend{},
	}
}

type ServerPool struct {
	alives []*Backend
	dead   []*Backend
}

func (p *ServerPool) Len() int {
	return len(p.alives)
}

func (p *ServerPool) Get(index int) *Backend {
	return p.alives[index]
}

func (p *ServerPool) Add(backend *Backend) {
	p.alives = append(p.alives, backend)
}

func (p *ServerPool) Remove(backend *Backend) {
	var index int
	for i, b := range p.alives {
		if b.rawUrl() == backend.rawUrl() {
			index = i
			break
		}
	}
	p.alives = append(p.alives[:index], p.alives[index+1:]...)
	p.dead = append(p.dead, backend)
}

func NewGateway() *Gateway {
	gtw := Gateway{
		serverPool: NewServerPool(),
		policy:     &RoundRobinPolicy{},
		syncCh:     make(chan struct{}),
		addCh:      make(chan *Backend),
		removeCh:   make(chan *Backend),
		nextCh:     make(chan *BackendResult),
	}

	go gtw.process()

	return &gtw
}

type BackendResult struct {
	resultCh chan<- *Backend
}

type Gateway struct {
	serverPool *ServerPool
	policy     Policy
	syncCh     chan struct{}
	addCh      chan *Backend
	removeCh   chan *Backend
	nextCh     chan *BackendResult
}

func (g *Gateway) process() {
	var backend *Backend
	var backendResult *BackendResult

	for {
		select {
		case <-g.syncCh:
			// no-op
		case backend = <-g.addCh:
			g.serverPool.Add(backend)
		case backend = <-g.removeCh:
			g.serverPool.Remove(backend)
		case backendResult = <-g.nextCh:
			if len(g.serverPool.alives) == 0 {
				backendResult.resultCh <- nil
			} else {
				backendResult.resultCh <- g.next()
			}
		}
	}
}

func (g *Gateway) sync() {
	g.syncCh <- struct{}{}
}

func (g *Gateway) AddServer(backend *Backend) {
	g.addCh <- backend
}

func (g *Gateway) RemoveServer(backend *Backend) {
	g.removeCh <- backend
}

func (g *Gateway) next() *Backend {
	return g.policy.Next(g.serverPool)
}

func (g *Gateway) NextServer() *Backend {
	resultCh := make(chan *Backend)
	backendResult := &BackendResult{
		resultCh: resultCh,
	}

	g.nextCh <- backendResult
	select {
	case backend := <-resultCh:
		return backend
	case <-time.After(100 * time.Millisecond):
	}

	return nil
}
