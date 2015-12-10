package ports

import (
	"fmt"
	"sync"
)

type PortPool struct {
	start uint32
	size  uint32

	pool      []uint32
	poolMutex sync.Mutex

	state State
}

type PoolExhaustedError struct{}

func (e PoolExhaustedError) Error() string {
	return "port pool is exhausted"
}

type PortTakenError struct {
	Port uint32
}

func (e PortTakenError) Error() string {
	return fmt.Sprintf("port already acquired: %d", e.Port)
}

func NewPool(start, size uint32, state State) (*PortPool, error) {
	if start+size > 65535 {
		return nil, fmt.Errorf("port_pool: New: invalid port range: startL %d, size: %d", start, size)
	}

	if state.Offset >= size {
		state.Offset = 0
	}

	pool := make([]uint32, size)

	i := 0
	for port := start + state.Offset; port < start+size; port++ {
		pool[i] = port
		i += 1
	}
	for port := start; port < start+state.Offset; port++ {
		pool[i] = port
		i += 1
	}

	return &PortPool{
		start: start,
		size:  size,

		pool: pool,
	}, nil
}

func (p *PortPool) Acquire() (uint32, error) {
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	if len(p.pool) == 0 {
		return 0, PoolExhaustedError{}
	}

	port := p.pool[0]

	p.pool = p.pool[1:]

	return port, nil
}

func (p *PortPool) Remove(port uint32) error {
	idx := 0
	found := false

	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	for i, existingPort := range p.pool {
		if existingPort == port {
			idx = i
			found = true
			break
		}
	}

	if !found {
		return PortTakenError{port}
	}

	p.pool = append(p.pool[:idx], p.pool[idx+1:]...)

	return nil
}

func (p *PortPool) Release(port uint32) {
	if port < p.start || port >= p.start+p.size {
		return
	}

	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	for _, existingPort := range p.pool {
		if existingPort == port {
			return
		}
	}

	p.pool = append(p.pool, port)
}

func (p *PortPool) RefreshState() State {
	if len(p.pool) == 0 {
		p.state.Offset = 0
	} else {
		p.state.Offset = p.pool[0] - p.start
	}
	return p.state
}
