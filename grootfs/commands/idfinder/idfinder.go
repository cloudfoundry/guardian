package idfinder

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/grootfs/store"
)

func FindID(storePath, pathOrID string) (string, error) {
	if !strings.HasPrefix(pathOrID, "/") {
		return pathOrID, nil
	}

	bundlesPath := filepath.Join(storePath, store.BUNDLES_DIR_NAME)
	if !strings.HasPrefix(pathOrID, bundlesPath) {
		return "", fmt.Errorf("path `%s` is outside store path", pathOrID)
	}

	dirtyID := strings.TrimPrefix(pathOrID, bundlesPath)
	return strings.Trim(dirtyID, "/"), nil
}
