package locksmith

import "os"

type Null struct {
}

func (l *Null) Lock(key string) (*os.File, error) {
	return nil, nil
}

func (l *Null) Unlock(lockFile *os.File) error {
	return nil
}
