package gateway

type Policy interface {
	Next(*ServerPool) *Backend
}

type RoundRobinPolicy struct {
	current int
}

func (rr *RoundRobinPolicy) Next(p *ServerPool) *Backend {
	backend := p.Get(rr.current % p.Len())
	rr.current = (rr.current + 1) % p.Len()
	return backend
}

// type LeastConnectionPolicy struct{}
