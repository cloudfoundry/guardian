package rundmc

import (
	"strings"
	"sync"
)

//go:generate counterfeiter . Properties

type Properties interface {
	Set(handle string, key string, value string)
	Get(handle string, key string) (string, error)
}

type events struct {
	props Properties
	mu    sync.Mutex
}

func NewEventStore(props Properties) *events {
	return &events{
		props: props,
	}
}

func (e *events) OnEvent(handle, event string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	events := append(e.Events(handle), event)
	e.props.Set(handle, "rundmc.events", strings.Join(events, ","))
}

func (e *events) Events(handle string) []string {
	if value, err := e.props.Get(handle, "rundmc.events"); err == nil && value != "" {
		return strings.Split(value, ",")
	}

	return nil
}
