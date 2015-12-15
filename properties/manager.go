package properties

import (
	"fmt"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
)

type Manager struct {
	propMutex sync.RWMutex
	prop      map[string]map[string]string
}

func NewManager() *Manager {
	return &Manager{
		prop: make(map[string]map[string]string),
	}
}

func (m *Manager) CreateKeySpace(handle string) error {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	if _, exists := m.prop[handle]; !exists {
		m.prop[handle] = make(map[string]string)
	}

	return nil
}

func (m *Manager) DestroyKeySpace(handle string) error {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	if _, exists := m.prop[handle]; !exists {
		return NoSuchKeySpaceError{
			Message: fmt.Sprintf("No such key space: %s", handle),
		}
	}

	delete(m.prop, handle)

	return nil
}

func (m *Manager) Set(handle string, name string, value string) error {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	m.prop[handle][name] = value
	return nil
}

func (m *Manager) All(handle string) (garden.Properties, error) {
	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	return m.prop[handle], nil
}

func (m *Manager) Get(handle string, name string) (string, error) {
	var (
		prop   string
		exists bool
	)

	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	if prop, exists = m.prop[handle][name]; !exists {
		return "", NoSuchPropertyError{
			Message: fmt.Sprintf("No such property: %s", name),
		}
	}

	return prop, nil
}

func (m *Manager) Remove(handle string, name string) error {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	if _, exists := m.prop[handle][name]; !exists {
		return NoSuchPropertyError{
			Message: fmt.Sprintf("No such property: %s", name),
		}
	}

	delete(m.prop[handle], name)

	return nil
}

func (m *Manager) MatchesAll(handle string, props garden.Properties) bool {
	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	for key, val := range props {
		if m.prop[handle][key] != val {
			return false
		}
	}

	return true
}

type NoSuchPropertyError struct {
	Message string
}

func (e NoSuchPropertyError) Error() string {
	return e.Message
}

type NoSuchKeySpaceError struct {
	Message string
}

func (e NoSuchKeySpaceError) Error() string {
	return e.Message
}
