package rundmc

import "sync"

type exitStore struct {
	mu       sync.Mutex
	channels map[string]<-chan struct{}
}

func NewExitStore() *exitStore {
	return &exitStore{channels: make(map[string]<-chan struct{})}
}

func (s *exitStore) Store(handle string, exit <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.channels[handle] = exit
}

func (s *exitStore) Unstore(handle string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.channels, handle)
}

func (s *exitStore) Wait(handle string) {
	if ch, ok := s.ch(handle); ok {
		<-ch
	}
}

func (s *exitStore) ch(handle string) (<-chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, ok := s.channels[handle]
	return ch, ok
}
