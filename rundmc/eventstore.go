package rundmc

import (
	"fmt"
	"strings"
	"sync"
)

//go:generate counterfeiter . Properties

type Properties interface {
	Set(handle string, key string, value string) error
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

func (e *events) OnEvent(handle, event string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	events := append(e.Events(handle), event)
	err := e.props.Set(handle, "rundmc.events", strings.Join(events, ","))
	if err != nil {
		return fmt.Errorf("failed to handle onEvent: %s", err)
	}

	return nil
}

func (e *events) Events(handle string) []string {
	if value, err := e.props.Get(handle, "rundmc.events"); err == nil && value != "" {
		return strings.Split(value, ",")
	}

	return nil
}
