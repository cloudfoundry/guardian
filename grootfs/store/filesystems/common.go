package filesystems

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

func WriteVolumeMeta(logger lager.Logger, storePath, id string, metadata base_image_puller.VolumeMeta) error {
	metaFile, err := os.Create(VolumeMetaFilePath(storePath, id))
	if err != nil {
		return errorspkg.Wrap(err, "creating metadata file")
	}

	if err = json.NewEncoder(metaFile).Encode(metadata); err != nil {
		return errorspkg.Wrap(err, "writing metadata file")
	}

	return nil
}

func VolumeSize(logger lager.Logger, storePath, id string) (int64, error) {
	metaFile, err := os.Open(VolumeMetaFilePath(storePath, id))
	if err != nil {
		return 0, errorspkg.Wrapf(err, "opening volume `%s` metadata", id)
	}

	var metadata base_image_puller.VolumeMeta
	err = json.NewDecoder(metaFile).Decode(&metadata)
	if err != nil {
		return 0, errorspkg.Wrapf(err, "parsing volume `%s` metadata", id)
	}

	return metadata.Size, nil
}

func VolumeMetaFilePath(storePath, id string) string {
	id = strings.Replace(id, "gc.", "", 1)
	return filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", id))
}
