package stats

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/grootfs/groot"
	quotapkg "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/quota"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

func VolumeStats(logger lager.Logger, imagePath string) (groot.VolumeStats, error) {
	logger = logger.Session("overlayxfs-fetching-stats", lager.Data{"imagePath": imagePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		logger.Error("image-path-not-found", err)
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "image path (%s) doesn't exist", imagePath)
	}

	projectID, err := quotapkg.GetProjectID(imagePath)
	if err != nil {
		logger.Error("fetching-project-id-failed", err)
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "fetching project id for %s", imagePath)
	}

	var exclusiveSize int64
	if projectID != 0 {
		exclusiveSize, err = listQuotaUsage(logger, imagePath)
		if err != nil {
			logger.Error("list-quota-usage-failed", err, lager.Data{"projectID": projectID})
			return groot.VolumeStats{}, errorspkg.Wrapf(err, "listing quota usage %s", imagePath)
		}
	}

	volumeSize, err := readImageInfo(logger, imagePath)
	if err != nil {
		logger.Error("reading-image-info-failed", err)
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "reading image info %s", imagePath)
	}

	logger.Debug("usage", lager.Data{"volumeSize": volumeSize, "exclusiveSize": exclusiveSize})

	return groot.VolumeStats{
		DiskUsage: groot.DiskUsage{
			ExclusiveBytesUsed: exclusiveSize,
			TotalBytesUsed:     volumeSize + exclusiveSize,
		},
	}, nil
}

func listQuotaUsage(logger lager.Logger, imagePath string) (int64, error) {
	logger = logger.Session("listing-quota-usage", lager.Data{"imagePath": imagePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	imagesPath := filepath.Dir(imagePath)
	quotaControl, err := quotapkg.NewControl(imagesPath)
	if err != nil {
		logger.Error("creating-quota-control-failed", err)
		return 0, errorspkg.Wrapf(err, "creating quota control")
	}

	var quota quotapkg.Quota
	if err := quotaControl.GetQuota(imagePath, &quota); err != nil {
		logger.Error("getting-quota-failed", err)
		return 0, errorspkg.Wrapf(err, "getting quota %s", imagePath)
	}

	return int64(quota.BCount), nil
}

func readImageInfo(logger lager.Logger, imagePath string) (int64, error) {
	contents, err := ioutil.ReadFile(filepath.Join(imagePath, "image_info"))
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(string(contents), 10, 64)
}
