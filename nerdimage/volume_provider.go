package nerdimage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/garden"
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

type ContainerdVolumizer struct {
	client   *containerd.Client
	context  context.Context
	storeDir string
	rootUID  int
	rootGID  int
}

func NewContainerdVolumizer(client *containerd.Client, context context.Context, storeDir string, rootUID, rootGID int) *ContainerdVolumizer {
	return &ContainerdVolumizer{client: client, context: context, storeDir: storeDir, rootUID: rootUID, rootGID: rootGID}
}

func (v ContainerdVolumizer) Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error) {
	image, err := v.client.Pull(v.context, spec.Image.URI, containerd.WithPullUnpack, containerd.WithPullLabel(spec.Handle, "set"))
	if err != nil {
		return specs.Spec{}, err
	}

	parentSnapshotId, err := getParentSnapshotID(v.context, image)
	if err != nil {
		return specs.Spec{}, err
	}

	noGarbageCollectLabel := map[string]string{
		"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339),
	}
	mnts, err := v.client.SnapshotService(containerd.DefaultSnapshotter).Prepare(v.context, spec.Handle, parentSnapshotId, snapshots.WithLabels(noGarbageCollectLabel))
	if err != nil {
		return specs.Spec{}, err
	}

	rootfsDir := filepath.Join(v.storeDir, spec.Handle)
	if err := os.MkdirAll(rootfsDir, 0775); err != nil {
		return specs.Spec{}, err
	}

	if err := mount.All(mnts, rootfsDir); err != nil {
		return specs.Spec{}, err
	}

	if err := recursiveChown(rootfsDir, v.rootUID, v.rootGID); err != nil {
		return specs.Spec{}, err
	}

	imgEnv, err := getImageEnvironment(v.context, image)
	if err != nil {
		return specs.Spec{}, err
	}

	// rootfsDir := filepath.Join("/var/vcap/data/containerd/state", spec.Handle, "rootfs")
	// return specs.Spec{Root: &specs.Root{Path: rootfsDir}}, nil
	return specs.Spec{Root: &specs.Root{Path: rootfsDir}, Process: &specs.Process{Env: imgEnv}}, nil
}

func (v ContainerdVolumizer) Destroy(log lager.Logger, handle string) error {

	// snapshotter := v.client.SnapshotService(containerd.DefaultSnapshotter)
	// rootfsDir := filepath.Join(v.storeDir, handle)
	// if err := unix.Unmount(rootfsDir, 0); err != nil {
	// 	return err
	// }

	// return snapshotter.Remove(v.context, handle)
	return nil
}

func (v ContainerdVolumizer) Metrics(log lager.Logger, handle string, namespaced bool) (garden.ContainerDiskStat, error) {
	return garden.ContainerDiskStat{}, nil
}

func (v ContainerdVolumizer) GC(log lager.Logger) error {
	return nil
}

func (v ContainerdVolumizer) Capacity(log lager.Logger) (uint64, error) {
	return 0, nil
}

func recursiveChown(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = os.Lchown(name, uid, gid)
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
