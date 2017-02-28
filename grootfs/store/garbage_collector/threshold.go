package garbage_collector

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

type StoreMeasurer struct {
	storePath string
}

func NewStoreMeasurer(storePath string) *StoreMeasurer {
	return &StoreMeasurer{
		storePath: storePath,
	}
}

func (s *StoreMeasurer) MeasureStore(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-store", lager.Data{"storePath": s.storePath})
	logger.Info("start")
	defer logger.Info("end")

	cacheSize, err := s.measurePath(filepath.Join(s.storePath, store.CacheDirName))
	if err != nil {
		return 0, err
	}
	logger.Info("got-cache-size", lager.Data{"cacheSize": cacheSize})

	volumesSize, err := s.measurePath(filepath.Join(s.storePath, store.VolumesDirName))
	if err != nil {
		return 0, err
	}
	logger.Info("got-volumes-size", lager.Data{"volumeSize": volumesSize})

	return cacheSize + volumesSize, nil
}

func (s *StoreMeasurer) measurePath(path string) (int64, error) {
	cmd := exec.Command("du", "-sb", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, errorspkg.Wrapf(err, "du for `%s` failed: %s", path, out)
	}

	return s.parseDuContents(string(out))
}

func (s *StoreMeasurer) parseDuContents(contents string) (int64, error) {
	parts := strings.Split(contents, "\t")
	if len(parts) != 2 {
		return 0, errorspkg.Errorf("failed to parse du's output `%s`", contents)
	}
	return strconv.ParseInt(parts[0], 10, 64)
}
