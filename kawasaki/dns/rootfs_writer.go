package dns

import (
	"os"
	"path/filepath"

	"github.com/pivotal-golang/lager"
)

var ChownFunc func(string, int, int) error = os.Chown

type RootfsWriter struct {
	RootfsPath string
	RootUid    int
	RootGid    int
}

func (r *RootfsWriter) WriteFile(log lager.Logger, filePath string, contents []byte) error {
	log = log.Session("rootfs-write-file", lager.Data{
		"rootfsPath": r.RootfsPath,
		"rootUid":    r.RootUid,
		"rootGit":    r.RootGid,
	})

	if _, err := os.Stat(r.RootfsPath); err != nil {
		log.Error("checking-rootfs-path", err)
		return err
	}

	filePath = filepath.Join(r.RootfsPath, filePath)
	parentDirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(parentDirPath, 0755); err != nil {
		log.Error("creating-directory", err)
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		log.Error("creating-file", err)
		return err
	}
	defer file.Close()

	if _, err := file.Write(contents); err != nil {
		log.Error("writting-file", err)
		return err
	}

	if err := ChownFunc(filePath, r.RootUid, r.RootGid); err != nil {
		log.Error("chowing-file", err)
		return err
	}

	return nil
}
