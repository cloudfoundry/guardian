package properties

import (
	"fmt"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
)

type Manager struct {
	propMutex sync.RWMutex
	prop      map[string]string
}

func NewManager() *Manager {
	return &Manager{
		prop: make(map[string]string),
	}
}

func (m *Manager) Set(name string, value string) error {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	m.prop[name] = value
	return nil
}

func (m *Manager) All() (garden.Properties, error) {
	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	return m.prop, nil
}

func (m *Manager) Get(name string) (string, error) {
	var (
		prop string
		ok   bool
	)

	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	if prop, ok = m.prop[name]; !ok {
		return "", NoSuchPropertyError{
			Message: fmt.Sprintf("No such property: %s", name),
		}
	}

	return prop, nil
}

func (m *Manager) Remove(name string) error {
	if _, ok := m.prop[name]; !ok {
		return NoSuchPropertyError{
			Message: fmt.Sprintf("No such property: %s", name),
		}
	}

	delete(m.prop, name)

	return nil
}

type NoSuchPropertyError struct {
	Message string
}

func (e NoSuchPropertyError) Error() string {
	return e.Message
}
