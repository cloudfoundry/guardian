package volume_driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
)

type Btrfs struct {
	storePath string
}

func NewBtrfs(storePath string) *Btrfs {
	return &Btrfs{
		storePath: storePath,
	}
}

func (d *Btrfs) Path(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", fmt.Errorf("volume does not exist `%s`: %s", id, err)
}

func (d *Btrfs) Create(logger lager.Logger, parentID, id string) (string, error) {
	logger = logger.Session("btrfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Debug("start")
	defer logger.Debug("end")

	var cmd *exec.Cmd
	volPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, id)
	if parentID == "" {
		cmd = exec.Command("btrfs", "subvolume", "create", volPath)
	} else {
		parentVolPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, parentID)
		cmd = exec.Command("btrfs", "subvolume", "snapshot", parentVolPath, volPath)
	}

	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf(
			"creating btrfs volume `%s` (%s): %s",
			volPath, err, string(contents),
		)
	}

	return volPath, nil
}

func (d *Btrfs) Snapshot(logger lager.Logger, id, targetPath string) error {
	logger = logger.Session("btrfs-creating-snapshot", lager.Data{"id": id, "targetPath": targetPath})
	logger.Debug("start")
	defer logger.Debug("end")

	volPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, id)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", volPath, targetPath)

	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"creating btrfs snapshot to `%s` (%s): %s",
			targetPath, err, string(contents),
		)
	}

	return nil
}
