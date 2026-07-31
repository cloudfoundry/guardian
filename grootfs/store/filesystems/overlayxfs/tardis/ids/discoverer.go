package ids

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager/v3"

	"github.com/pkg/errors"
)

func NewDiscoverer(idsPath string) *Discoverer {
	return &Discoverer{
		idsPath: idsPath,
	}
}

type Discoverer struct {
	idsPath string
}

func (i *Discoverer) Alloc(logger lager.Logger) (projId uint32, err error) {
	logger = logger.Session("project-id-allocation")
	logger.Debug("starting")
	defer func() {
		logger.Debug("ending", lager.Data{"projectID": projId})
	}()

	contents, err := os.ReadDir(i.idsPath)
	if err != nil {
		return 0, errors.Wrap(err, "reading directory")
	}

	nextId := len(contents) + 1
	// #nosec G115 - length of an array can't be negative.
	return i.untilSucceeds(uint32(nextId))
}

func (i *Discoverer) untilSucceeds(startId uint32) (uint32, error) {
	if startId == 1 {
		startId++
	}

	for {
		if err := os.Mkdir(filepath.Join(i.idsPath, fmt.Sprintf("%d", startId)), 0755); err != nil {
			if os.IsExist(err) {
				startId++
			} else {
				return 0, errors.Wrap(err, "failed to create id file")
			}
		} else {
			break
		}
	}
	return uint32(startId), nil
}
