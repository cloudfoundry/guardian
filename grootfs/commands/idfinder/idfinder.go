package idfinder

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/grootfs/store"
)

func FindID(storePath, pathOrID string) (string, error) {
	if !strings.HasPrefix(pathOrID, "/") {
		return pathOrID, nil
	}

	pathRegexp := regexp.MustCompile(filepath.Join(storePath, "[0-9]*", store.IMAGES_DIR_NAME, "(.*)"))
	matches := pathRegexp.FindStringSubmatch(pathOrID)

	if len(matches) != 2 {
		return "", fmt.Errorf("path `%s` is outside store path", pathOrID)
	}

	return matches[1], nil
}

func FindSubStorePath(storePath, path string) (string, error) {
	pathRegexp := regexp.MustCompile(filepath.Join(storePath, "([0-9]*)", store.IMAGES_DIR_NAME, ".*"))
	matches := pathRegexp.FindStringSubmatch(path)

	if len(matches) != 2 {
		return "", fmt.Errorf("unable to match substore in path `%s`", path)
	}

	return filepath.Join(storePath, matches[1]), nil
}
