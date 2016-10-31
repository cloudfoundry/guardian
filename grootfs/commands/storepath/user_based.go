package storepath

import (
	"os"
	"path/filepath"
	"strconv"
)

func UserBased(storePath string) string {
	userID := os.Getuid()
	return filepath.Join(storePath, strconv.Itoa(userID))
}
