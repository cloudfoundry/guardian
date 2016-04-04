package properties

import (
	"fmt"

	"github.com/cloudfoundry-incubator/garden"
)

//go:generate counterfeiter . MapPersister
type MapPersister interface {
	LoadMap(string) (map[string]string, error)
	SaveMap(string, map[string]string) error
	DeleteMap(string) error
	IsMapPersisted(string) bool
}

type Manager struct {
	mapPersister MapPersister
}

func NewManager(mapPersister MapPersister) *Manager {
	return &Manager{
		mapPersister: mapPersister,
	}
}

func (m *Manager) DestroyKeySpace(handle string) error {
	if !m.mapPersister.IsMapPersisted(handle) {
		return nil
	}

	return m.mapPersister.DeleteMap(handle)
}

func (m *Manager) Set(handle string, name string, value string) error {
	propMap := map[string]string{}
	var err error
	if m.mapPersister.IsMapPersisted(handle) {
		propMap, err = m.mapPersister.LoadMap(handle)
		if err != nil {
			return fmt.Errorf("failed to set property for handle: %s - %s", handle, err)
		}
	}

	propMap[name] = value

	err = m.mapPersister.SaveMap(handle, propMap)
	if err != nil {
		return fmt.Errorf("failed to set property for handle: %s - %s", handle, err)
	}

	return nil
}

func (m *Manager) All(handle string) (garden.Properties, error) {
	props, err := m.mapPersister.LoadMap(handle)
	if err != nil {
		return map[string]string{}, fmt.Errorf("failed to get properties for handle: %s - %s", handle, err)
	}

	return props, nil
}

func (m *Manager) Get(handle string, name string) (string, error) {
	var (
		prop   string
		exists bool
	)

	propMap, err := m.mapPersister.LoadMap(handle)
	if err != nil {
		return "", fmt.Errorf("cannot Get %s:%s - %s", handle, name, err)
	}

	if prop, exists = propMap[name]; !exists {
		return "", NoSuchPropertyError{
			Message: fmt.Sprintf("cannot Get %s:%s", handle, name),
		}
	}

	return prop, nil
}

func (m *Manager) Remove(handle string, name string) error {
	propMap, err := m.mapPersister.LoadMap(handle)
	if err != nil {
		return fmt.Errorf("cannot Remove %s:%s - %s", handle, name, err)
	}

	if _, exists := propMap[name]; !exists {
		return NoSuchPropertyError{
			Message: fmt.Sprintf("cannot Remove %s:%s", handle, name),
		}
	}

	delete(propMap, name)

	err = m.mapPersister.SaveMap(handle, propMap)
	if err != nil {
		return fmt.Errorf("cannot Remove %s:%s - %s", handle, name, err)
	}

	return nil
}

func (m *Manager) MatchesAll(handle string, props garden.Properties) (bool, error) {
	if len(props) == 0 {
		return true, nil
	}

	if !m.mapPersister.IsMapPersisted(handle) {
		return false, nil
	}

	propMap, err := m.mapPersister.LoadMap(handle)
	if err != nil {
		return false, fmt.Errorf("cannot MatchAll %s - %s", handle, err)
	}

	for key, val := range props {
		if propMap[key] != val {
			return false, nil
		}
	}

	return true, nil
}

type NoSuchPropertyError struct {
	Message string
}

func (e NoSuchPropertyError) Error() string {
	return e.Message
}
