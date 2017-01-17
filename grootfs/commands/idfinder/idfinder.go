package idfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/grootfs/store"
)

func FindID(storePath string, userID int, pathOrID string) (string, error) {
	var (
		imageID   string
		imagePath string
		usrID     string
	)

	usrID = fmt.Sprintf("%d", userID)
	if !strings.HasPrefix(pathOrID, "/") {
		imageID = pathOrID
		imagePath = filepath.Join(storePath, usrID, store.IMAGES_DIR_NAME, imageID)
	} else {
		imagePathRegex := filepath.Join(storePath, usrID, store.IMAGES_DIR_NAME, "(.*)")
		pathRegexp := regexp.MustCompile(imagePathRegex)
		matches := pathRegexp.FindStringSubmatch(pathOrID)
		if len(matches) != 2 {
			return "", fmt.Errorf("path `%s` is outside store path", pathOrID)
		}
		imageID = matches[1]
		imagePath = pathOrID
	}

	if !exists(imagePath) {
		return "", fmt.Errorf("image `%s` was not found", imageID)
	}

	return imageID, nil
}

func FindSubStorePath(storePath, path string) (string, error) {
	pathRegexp := regexp.MustCompile(filepath.Join(storePath, "([0-9]*)", store.IMAGES_DIR_NAME, ".*"))
	matches := pathRegexp.FindStringSubmatch(path)

	if len(matches) != 2 {
		return "", fmt.Errorf("unable to match substore in path `%s`", path)
	}

	return filepath.Join(storePath, matches[1]), nil
}

func exists(imagePath string) bool {
	if _, err := os.Stat(imagePath); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
