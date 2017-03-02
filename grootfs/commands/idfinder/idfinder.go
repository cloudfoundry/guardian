package idfinder

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/grootfs/store"
	errorspkg "github.com/pkg/errors"
)

func FindID(storePath string, pathOrID string) (string, error) {
	var (
		imageID   string
		imagePath string
	)

	if !strings.HasPrefix(pathOrID, "/") {
		imageID = pathOrID
		imagePath = filepath.Join(storePath, store.ImageDirName, imageID)
	} else {
		imagePathRegex := filepath.Join(storePath, store.ImageDirName, "(.*)")
		pathRegexp := regexp.MustCompile(imagePathRegex)
		matches := pathRegexp.FindStringSubmatch(pathOrID)

		if len(matches) != 2 {
			return "", errorspkg.Errorf("path `%s` is outside store path", pathOrID)
		}
		imageID = matches[1]
		imagePath = pathOrID
	}

	if !exists(imagePath) {
		return "", errorspkg.Errorf("Image `%s` not found. Skipping delete.", imageID)
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
