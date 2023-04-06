package ids

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

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

	contents, err := ioutil.ReadDir(i.idsPath)
	if err != nil {
		return 0, errors.Wrap(err, "reading directory")
	}

	nextId := len(contents) + 1
	return i.untilSucceeds(nextId)
}

func (i *Discoverer) untilSucceeds(startId int) (uint32, error) {
	if startId == 1 {
		startId++
	}

	for {
		if err := os.Mkdir(filepath.Join(i.idsPath, strconv.Itoa(startId)), 0755); err != nil {
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
