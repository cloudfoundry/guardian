package properties

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
)

type FilesystemPersister struct {
	fileMutex      sync.RWMutex
	PersistenceDir string
}

func (p *FilesystemPersister) LoadMap(filename string) (map[string]string, error) {
	p.fileMutex.RLock()
	defer p.fileMutex.RUnlock()

	file, err := os.Open(path.Join(p.PersistenceDir, filename))
	if err != nil {
		return map[string]string{}, fmt.Errorf("opening file: %s", err)
	}
	defer file.Close()

	var props map[string]string
	if err := json.NewDecoder(file).Decode(&props); err != nil {
		return map[string]string{}, fmt.Errorf("parsing file: %s", err)
	}

	return props, nil
}

func (p *FilesystemPersister) SaveMap(filename string, props map[string]string) error {
	p.fileMutex.Lock()
	defer p.fileMutex.Unlock()

	file, err := os.Create(path.Join(p.PersistenceDir, filename))
	if err != nil {
		return fmt.Errorf("creating file: %s", err)
	}
	defer file.Close()

	json.NewEncoder(file).Encode(props)
	return nil
}

func (p *FilesystemPersister) DeleteMap(filename string) error {
	p.fileMutex.Lock()
	defer p.fileMutex.Unlock()

	err := os.Remove(path.Join(p.PersistenceDir, filename))
	if err != nil {
		return fmt.Errorf("deleting file: %s", err)
	}

	return nil
}

func (p *FilesystemPersister) IsMapPersisted(filename string) bool {
	p.fileMutex.RLock()
	defer p.fileMutex.RUnlock()

	_, err := os.Stat(path.Join(p.PersistenceDir, filename))
	return !os.IsNotExist(err)
}
