package storepath

import (
	"fmt"
	"os/user"
	"path/filepath"
)

func UserBased(storePath string) (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("fetching current user: %s", err)
	}

	return filepath.Join(storePath, currentUser.Uid), nil
}
