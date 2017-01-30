package idfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/grootfs/store"
)

func FindID(storePath string, pathOrID string) (string, error) {
	var (
		imageID   string
		imagePath string
	)

	if !strings.HasPrefix(pathOrID, "/") {
		imageID = pathOrID
		imagePath = filepath.Join(storePath, store.IMAGES_DIR_NAME, imageID)
	} else {
		imagePathRegex := filepath.Join(storePath, store.IMAGES_DIR_NAME, "(.*)")
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

func exists(imagePath string) bool {
	if _, err := os.Stat(imagePath); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
