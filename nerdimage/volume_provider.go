package nerdimage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const ociRootfsStore = "/var/vcap/data/nerdimage/"

type ContainerdVolumizer struct {
	client           *containerd.Client
	context          context.Context
	defaultRootfs    string
	storeDir         string
	uidMappings      idmapper.MappingList
	gidMappings      idmapper.MappingList
	imageSpecCreator ImageSpecCreator
	mutex            sync.Mutex
}

//go:generate counterfeiter . ImageSpecCreator
type ImageSpecCreator interface {
	CreateImageSpec(rootFS *url.URL, handle string) (*url.URL, error)
}

func NewContainerdVolumizer(client *containerd.Client, context context.Context, defaultRootfs, storeDir string, uidMappings, gidMappings idmapper.MappingList, imageSpecCreator ImageSpecCreator) *ContainerdVolumizer {
	err := os.MkdirAll(ociRootfsStore, 0755)
	if err != nil {
		panic(err)
	}
	err = os.Lchown(ociRootfsStore, uidMappings.Map(0), gidMappings.Map(0))
	if err != nil {
		panic(err)
	}

	return &ContainerdVolumizer{
		client:           client,
		context:          context,
		defaultRootfs:    defaultRootfs,
		storeDir:         ociRootfsStore,
		uidMappings:      uidMappings,
		gidMappings:      gidMappings,
		imageSpecCreator: imageSpecCreator,
	}
}

func (v *ContainerdVolumizer) Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error) {
	log.Info("volumizer-create-spec", lager.Data{"spec": spec})

	switch {
	case strings.Contains(spec.Image.URI, "docker"):
		image, err := v.client.Pull(v.context, spec.Image.URI, containerd.WithPullUnpack, containerd.WithPullLabel(spec.Handle, "set"))
		if err != nil {
			return specs.Spec{}, err
		}
		return v.containerSpecFromImage(log, image, spec.Handle)

	case strings.Contains(spec.Image.URI, "preloaded+layer"):

		rootFSURL, err := url.Parse(spec.Image.URI)
		if err != nil {
			return specs.Spec{}, err
		}
		ociImageURL, err := v.imageSpecCreator.CreateImageSpec(rootFSURL, spec.Handle)
		if err != nil {
			return specs.Spec{}, err
		}

		blobstoreResolver := NewBlobstoreResolver(ociImageURL.Path, spec.Handle+"-unmapped")
		image, err := v.client.Pull(v.context, spec.Handle+"-unmapped", containerd.WithPullUnpack, containerd.WithPullLabel(spec.Handle, "set"), containerd.WithResolver(blobstoreResolver))
		if err != nil {
			return specs.Spec{}, err
		}
		return v.containerSpecFromImage(log, image, spec.Handle)

		// tarDir, err := ioutil.TempDir("", "")
		// if err != nil {
		// 	return specs.Spec{}, err
		// }
		// defer os.RemoveAll(tarDir)
		// tarPath := filepath.Join(tarDir, "image.tar")
		// err = exec.Command("tar", "-C", ociImageURL.Path, tarPath, ".").Run()
		// if err != nil {
		// 	return specs.Spec{}, err
		// }

		// tarFile, err := os.OpenFile(tarPath, os.O_RDONLY, 0)
		// if err != nil {
		// 	return specs.Spec{}, err
		// }
		// defer tarFile.Close()

		// images, err := v.client.Import(v.context, tarFile)
		// if err != nil {
		// 	return specs.Spec{}, err
		// }

		// if len(images) != 1 {
		// 	return specs.Spec{}, fmt.Errorf("expected one image, received %d", len(images))
		// }

		// // The image returned by import is not the same type returned by client.Pull...
		// nerdImage, err := v.client.GetImage(v.context, images[0].Name)
		// if err != nil {
		// 	return specs.Spec{}, err
		// }

		// return v.containerSpecFromImage(log, nerdImage, spec.Handle)

	default:
		err := v.createLocalRootfs(spec)
		if err != nil {
			return specs.Spec{}, err
		}

		mounts, err := v.client.SnapshotService(containerd.DefaultSnapshotter).Prepare(v.context, spec.Handle, "local-rootfs", snapshots.WithLabels(noGarbageCollectLabel()))
		if err != nil {
			return specs.Spec{}, err
		}

		rootfsDir := filepath.Join(v.storeDir, spec.Handle)
		if err := os.MkdirAll(rootfsDir, 0775); err != nil {
			return specs.Spec{}, err
		}

		if err := mount.All(mounts, rootfsDir); err != nil {
			return specs.Spec{}, err
		}

		// rootfsDir := filepath.Join("/var/vcap/data/containerd/state", spec.Handle, "rootfs")
		// return specs.Spec{Root: &specs.Root{Path: rootfsDir}}, nil
		return specs.Spec{Root: &specs.Root{Path: rootfsDir}, Process: &specs.Process{Env: []string{}}}, nil
	}

}

func (v *ContainerdVolumizer) createLocalRootfs(spec garden.ContainerSpec) error {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if _, err := v.client.SnapshotService(containerd.DefaultSnapshotter).Stat(v.context, "local-rootfs"); err != nil {
		// WHY DOES THIS NOT WORK WITH errdefs.IsNotFound(err) !?
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("Stat: %w", err)
		}

		mnts, err := v.client.SnapshotService(containerd.DefaultSnapshotter).Prepare(v.context, "random", "", snapshots.WithLabels(noGarbageCollectLabel()))
		if err != nil {
			return fmt.Errorf("Prepare: %w", err)
		}

		tempDir, err := ioutil.TempDir(os.TempDir(), "boo")
		if err != nil {
			return fmt.Errorf("TempDir: %w", err)
		}

		if err := mount.All(mnts, tempDir); err != nil {
			return fmt.Errorf("mount.All: %w", err)
		}

		// we could also untar the rootfs tar into this folder, this is quick hack with copy-pasted code
		var rootFsPath string
		if len(spec.RootFSPath) > 0 {
			rootFsPath = spec.RootFSPath
		} else {
			rootFsPath = spec.Image.URI
		}
		err = exec.Command("tar", "-x", "-f", rootFsPath, "-C", tempDir).Run()
		if err != nil {
			return fmt.Errorf("tar -x -f %s -C %s: %w [spec: %#v]", rootFsPath, tempDir, err, spec)
		}

		if err := v.recursiveChown(tempDir); err != nil {
			return fmt.Errorf("chown: %w", err)
		}

		if err := v.client.SnapshotService(containerd.DefaultSnapshotter).Commit(v.context, "local-rootfs", "random"); err != nil {
			return fmt.Errorf("Commit: %w", err)
		}
	}

	return nil
}

func (v *ContainerdVolumizer) containerSpecFromImage(log lager.Logger, image containerd.Image, handle string) (specs.Spec, error) {
	parentId, err := getParentSnapshotID(v.context, image)
	if err != nil {
		return specs.Spec{}, err
	}

	mnts, err := v.client.SnapshotService(containerd.DefaultSnapshotter).Prepare(v.context, handle, parentId, snapshots.WithLabels(noGarbageCollectLabel()))
	if err != nil {
		return specs.Spec{}, err
	}

	rootfsDir := filepath.Join(v.storeDir, handle)
	if err := os.MkdirAll(rootfsDir, 0775); err != nil {
		return specs.Spec{}, err
	}
	err = os.Lchown(rootfsDir, v.uidMappings.Map(0), v.gidMappings.Map(0))
	if err != nil {
		return specs.Spec{}, err
	}

	if err := mount.All(mnts, rootfsDir); err != nil {
		return specs.Spec{}, err
	}

	if err := v.recursiveChown(rootfsDir); err != nil {
		return specs.Spec{}, fmt.Errorf("chown: %w", err)
	}

	imgEnv, err := getImageEnvironment(v.context, image)
	if err != nil {
		return specs.Spec{}, err
	}

	return specs.Spec{Root: &specs.Root{Path: rootfsDir}, Process: &specs.Process{Env: imgEnv}}, nil
}

func (v *ContainerdVolumizer) Destroy(log lager.Logger, handle string) error {

	// snapshotter := v.client.SnapshotService(containerd.DefaultSnapshotter)
	// rootfsDir := filepath.Join(v.storeDir, handle)
	// if err := unix.Unmount(rootfsDir, 0); err != nil {
	// 	return err
	// }

	// return snapshotter.Remove(v.context, handle)
	return nil
}

func (v *ContainerdVolumizer) Metrics(log lager.Logger, handle string, namespaced bool) (garden.ContainerDiskStat, error) {
	return garden.ContainerDiskStat{}, nil
}

func (v *ContainerdVolumizer) GC(log lager.Logger) error {
	return nil
}

func (v *ContainerdVolumizer) Capacity(log lager.Logger) (uint64, error) {
	return 0, nil
}

func (v *ContainerdVolumizer) recursiveChown(path string) error {
	uids := map[int]int{}
	gids := map[int]int{}
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			stat := info.Sys().(*syscall.Stat_t)
			u, g := int(stat.Uid), int(stat.Gid)
			if _, ok := uids[u]; !ok {
				uids[u] = v.uidMappings.Map(u)
			}
			if _, ok := gids[u]; !ok {
				gids[g] = v.gidMappings.Map(g)
			}
			if u == uids[u] && g == gids[g] {
				return nil
			}
			err = os.Lchown(name, uids[u], gids[g])
		}
		return err
	})
}

func getParentSnapshotID(ctx context.Context, image containerd.Image) (string, error) {
	diffIDs, err := image.RootFS(ctx)
	if err != nil {
		return "", err
	}

	return identity.ChainID(diffIDs).String(), nil
}

// This is copied from containerd's spec_opts.go/WithImageConfigArgs
func getImageEnvironment(ctx context.Context, image containerd.Image) ([]string, error) {
	ic, err := image.Config(ctx)
	if err != nil {
		return nil, err
	}
	var (
		ociimage v1.Image
		config   v1.ImageConfig
	)
	switch ic.MediaType {
	case v1.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
		p, err := content.ReadBlob(ctx, image.ContentStore(), ic)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(p, &ociimage); err != nil {
			return nil, err
		}
		config = ociimage.Config
	default:
		return nil, fmt.Errorf("unknown image config media type %s", ic.MediaType)
	}

	return config.Env, nil
}

// Dir copies a whole directory recursively
func copyDir(src string, dst string) error {
	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	if fds, err = ioutil.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = copyDir(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		} else {
			if err = copyFile(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

// File copies a single file from src to dst
func copyFile(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func noGarbageCollectLabel() map[string]string {
	return map[string]string{"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339)}
}
