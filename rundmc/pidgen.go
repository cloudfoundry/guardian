package rundmc

import "sync"

type SimplePidGenerator struct {
	mu   sync.Mutex
	next uint32
}

func (s *SimplePidGenerator) Generate() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.next++
	return s.next
}
