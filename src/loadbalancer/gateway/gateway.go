package gateway

import (
	"log"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

var DefaultGateway = NewGateway()

func Reset() {
	DefaultGateway = NewGateway()
}

func NewBackend(rawUrl string, gtw *Gateway) *Backend {
	serverUrl, err := url.Parse(rawUrl)
	if err != nil {
		return nil
	}

	proxy := httputil.NewSingleHostReverseProxy(serverUrl)
	proxy.ErrorHandler = genRetryErrorHandler(serverUrl, proxy, gtw)

	return &Backend{
		URL:          serverUrl,
		ReverseProxy: proxy,
	}
}

type Backend struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy
}

func (b *Backend) rawUrl() string {
	return b.URL.String()
}

func (b *Backend) Url() *url.URL {
	return b.URL
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

func (p *ServerPool) Add(backends ...*Backend) {
	p.alives = append(p.alives, backends...)
}

func (p *ServerPool) Remove(rawUrl string) {
	var index int
	for i, b := range p.alives {
		if rawUrl == b.rawUrl() {
			index = i
			break
		}
	}
	backend := p.alives[index]
	p.alives = append(p.alives[:index], p.alives[index+1:]...)
	p.dead = append(p.dead, backend)
}

func (p *ServerPool) Delete(rawUrl string) {
	var index int
	for i, b := range p.dead {
		if rawUrl == b.rawUrl() {
			index = i
			break
		}
	}
	p.dead = append(p.dead[:index], p.dead[index+1:]...)
}

func NewGateway() *Gateway {
	gtw := Gateway{
		serverPool: NewServerPool(),
		policy:     &RoundRobinPolicy{},
		syncCh:     make(chan struct{}),
		addCh:      make(chan *Backend),
		removeCh:   make(chan string),
		nextCh:     make(chan *BackendResult),
		copyCh:     make(chan *CopyResult),
		deleteCh:   make(chan *DeleteResult),
	}

	go gtw.process()

	return &gtw
}

type CopyResult struct {
	resultCh chan []*Backend
}

type DeleteResult struct {
	resultCh chan []*Backend
}

type BackendResult struct {
	resultCh chan<- *Backend
}

type Gateway struct {
	serverPool *ServerPool
	policy     Policy
	syncCh     chan struct{}
	addCh      chan *Backend
	removeCh   chan string
	deleteCh   chan *DeleteResult
	copyCh     chan *CopyResult
	nextCh     chan *BackendResult
}

func (g *Gateway) process() {
	var backend *Backend
	var rawUrl string
	var backendResult *BackendResult
	var copyResult *CopyResult
	var deleteResult *DeleteResult

	for {
		select {
		case <-g.syncCh:
			// no-op

		case copyResult = <-g.copyCh:
			copied := make([]*Backend, g.serverPool.Len())
			copy(copied, g.serverPool.alives)
			copyResult.resultCh <- copied

		case deleteResult = <-g.deleteCh:
			deleted := g.serverPool.dead
			g.serverPool.dead = []*Backend{}
			deleteResult.resultCh <- deleted

		case backend = <-g.addCh:
			g.serverPool.Add(backend)

		case rawUrl = <-g.removeCh:
			g.serverPool.Remove(rawUrl)

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

func (g *Gateway) next() *Backend {
	return g.policy.Select(g.serverPool)
}

func (g *Gateway) copy() []*Backend {
	copyResult := CopyResult{
		resultCh: make(chan []*Backend),
	}
	g.copyCh <- &copyResult
	return <-copyResult.resultCh
}

func (g *Gateway) healthCheck() {
	backends := g.copy()
	var wg sync.WaitGroup
	wg.Add(len(backends))
	for _, b := range backends {
		go func(u *url.URL) {
			alive := isBackendAlive(u.String() + "/health_check")
			if !alive {
				g.RemoveServer(u.String())
			}
			wg.Done()
		}(b.URL)
	}
	wg.Wait()
}

func (g *Gateway) healthCheckAsync() {
	backends := g.copy()
	for _, b := range backends {
		go func(u *url.URL) {
			alive := isBackendAlive(u.String() + "/health_check")
			if !alive {
				g.RemoveServer(u.String())
			}
		}(b.URL)
	}
}

func (g *Gateway) report() error {
	// send to monitor for alive servers
	deleteResult := DeleteResult{
		resultCh: make(chan []*Backend),
	}

	g.deleteCh <- &deleteResult
	deleted := <-deleteResult.resultCh

	// TODO: send deleted servers to the monitor
	log.Printf("Removed Servers:%d\n", len(deleted))

	deletedServers := make([]string, len(deleted))
	for _, v := range deleted {
		deletedServers = append(deletedServers, v.rawUrl())
	}

	err := reportDeadBackends("http://localhost:3000/report", deletedServers)

	return err
}

func (g *Gateway) AddServer(backend *Backend) {
	g.AddServerAsync(backend)
	g.sync()
}

func (g *Gateway) AddServerAsync(backend *Backend) {
	g.addCh <- backend
}

func (g *Gateway) RemoveServer(rawUrl string) {
	g.RemoveServerAsync(rawUrl)
	g.sync()
}

func (g *Gateway) RemoveServerAsync(rawUrl string) {
	g.removeCh <- rawUrl
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
