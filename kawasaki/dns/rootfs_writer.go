package dns

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/docker/docker/pkg/reexec"
)

type RootfsWriter struct {
	RootfsPath string
	RootUid    int
	RootGid    int
}

func init() {
	reexec.Register("chrootwrite", func() {
		var rootfs, path string
		var uid, gid int
		flag.StringVar(&rootfs, "rootfs", "", "rootfs")
		flag.StringVar(&path, "path", "", "path")
		flag.IntVar(&uid, "uid", 0, "uid")
		flag.IntVar(&gid, "gid", 0, "gid")
		flag.Parse()

		if err := syscall.Chroot(rootfs); err != nil {
			panic(err)
		}

		if err := os.Chdir("/"); err != nil {
			panic(err)
		}

		var contents bytes.Buffer
		if _, err := io.Copy(&contents, os.Stdin); err != nil {
			panic(err)
		}

		w := RootfsWriter{
			RootfsPath: rootfs,
			RootUid:    uid,
			RootGid:    gid,
		}

		if err := w.writeFile(lager.NewLogger("chroot-write"), path, contents.Bytes()); err != nil {
			panic(err)
		}
	})
}

func (r *RootfsWriter) WriteFile(log lager.Logger, filePath string, contents []byte) error {
	cmd := reexec.Command("chrootwrite",
		"-rootfs", r.RootfsPath,
		"-path", filePath,
		"-uid", strconv.Itoa(r.RootUid),
		"-gid", strconv.Itoa(r.RootGid),
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	if _, err = stdin.Write(contents); err != nil {
		return err
	}

	if err = stdin.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

func (r *RootfsWriter) writeFile(log lager.Logger, filePath string, contents []byte) error {
	log = log.Session("rootfs-write-file", lager.Data{
		"rootfs":  r.RootfsPath,
		"path":    filePath,
		"rootUid": r.RootUid,
		"rootGit": r.RootGid,
	})

	log.Info("write")

	dir := filepath.Dir(filePath)
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0755)
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

	if err := os.Chown(filePath, r.RootUid, r.RootGid); err != nil {
		log.Error("chowing-file", err)
		return err
	}

	log.Info("written")
	return nil
}
