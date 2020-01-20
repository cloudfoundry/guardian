// +build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package containerd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
)

func WithUsernamespacedSnapshot(id string, i Image, uidMappings, gidMappings []IDMapping) NewContainerOpts {
	return func(ctx context.Context, client *Client, c *containers.Container) error {
		diffIDs, err := i.(*image).i.RootFS(ctx, client.ContentStore(), client.platform)
		if err != nil {
			return err
		}

		var (
			parent   = identity.ChainID(diffIDs).String()
			usernsID = fmt.Sprintf("%s-%s-%s", parent, IDMappingsRef(uidMappings), IDMappingsRef(gidMappings))
		)
		c.Snapshotter, err = client.resolveSnapshotterName(ctx, c.Snapshotter)
		if err != nil {
			return err
		}
		snapshotter, err := client.getSnapshotter(ctx, c.Snapshotter)
		if err != nil {
			return err
		}
		if _, err := snapshotter.Stat(ctx, usernsID); err == nil {
			if _, err := snapshotter.Prepare(ctx, id, usernsID); err == nil {
				c.SnapshotKey = id
				c.Image = i.Name()
				return nil
			} else if !errdefs.IsNotFound(err) {
				return err
			}
		}
		mounts, err := snapshotter.Prepare(ctx, usernsID+"-remap", parent, snapshots.WithLabels(map[string]string{"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339)}))
		if err != nil {
			return err
		}
		if err := recursiveChown(ctx, mounts, uidMappings, gidMappings); err != nil {
			snapshotter.Remove(ctx, usernsID)
			return err
		}
		if err := snapshotter.Commit(ctx, usernsID, usernsID+"-remap"); err != nil {
			return err
		}
		_, err = snapshotter.Prepare(ctx, id, usernsID)
		if err != nil {
			return err
		}
		c.SnapshotKey = id
		c.Image = i.Name()
		return nil
	}
}

// WithRemappedSnapshot creates a new snapshot and remaps the uid/gid for the
// filesystem to be used by a container with user namespaces
func WithRemappedSnapshot(id string, i Image, uid, gid uint32) NewContainerOpts {
	return withRemappedSnapshotBase(id, i, uid, gid, false)
}

// WithRemappedSnapshotView is similar to WithRemappedSnapshot but rootfs is mounted as read-only.
func WithRemappedSnapshotView(id string, i Image, uid, gid uint32) NewContainerOpts {
	return withRemappedSnapshotBase(id, i, uid, gid, true)
}

func withRemappedSnapshotBase(id string, i Image, uid, gid uint32, readonly bool) NewContainerOpts {
	return func(ctx context.Context, client *Client, c *containers.Container) error {
		diffIDs, err := i.(*image).i.RootFS(ctx, client.ContentStore(), client.platform)
		if err != nil {
			return err
		}

		var (
			parent   = identity.ChainID(diffIDs).String()
			usernsID = fmt.Sprintf("%s-%d-%d", parent, uid, gid)
		)
		c.Snapshotter, err = client.resolveSnapshotterName(ctx, c.Snapshotter)
		if err != nil {
			return err
		}
		snapshotter, err := client.getSnapshotter(ctx, c.Snapshotter)
		if err != nil {
			return err
		}
		if _, err := snapshotter.Stat(ctx, usernsID); err == nil {
			if _, err := snapshotter.Prepare(ctx, id, usernsID); err == nil {
				c.SnapshotKey = id
				c.Image = i.Name()
				return nil
			} else if !errdefs.IsNotFound(err) {
				return err
			}
		}
		mounts, err := snapshotter.Prepare(ctx, usernsID+"-remap", parent)
		if err != nil {
			return err
		}
		if err := remapRootFS(ctx, mounts, uid, gid); err != nil {
			snapshotter.Remove(ctx, usernsID)
			return err
		}
		if err := snapshotter.Commit(ctx, usernsID, usernsID+"-remap"); err != nil {
			return err
		}
		if readonly {
			_, err = snapshotter.View(ctx, id, usernsID)
		} else {
			_, err = snapshotter.Prepare(ctx, id, usernsID)
		}
		if err != nil {
			return err
		}
		c.SnapshotKey = id
		c.Image = i.Name()
		return nil
	}
}

func remapRootFS(ctx context.Context, mounts []mount.Mount, uid, gid uint32) error {
	return mount.WithTempMount(ctx, mounts, func(root string) error {
		return filepath.Walk(root, incrementFS(root, uid, gid))
	})
}

func incrementFS(root string, uidInc, gidInc uint32) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		var (
			stat = info.Sys().(*syscall.Stat_t)
			u, g = int(stat.Uid + uidInc), int(stat.Gid + gidInc)
		)
		// be sure the lchown the path as to not de-reference the symlink to a host file
		return os.Lchown(path, u, g)
	}
}

type IDMapping struct {
	// ContainerID is the starting UID/GID in the container
	ContainerID uint32 `json:"containerID"`
	// HostID is the starting UID/GID on the host to be mapped to 'ContainerID'
	HostID uint32 `json:"hostID"`
	// Size is the number of IDs to be mapped
	Size uint32 `json:"size"`
}

func IDMappingsRef(mappings []IDMapping) string {
	list := []string{}
	for _, mapping := range mappings {
		list = append(list, fmt.Sprintf("%s-%s-%s", mapping.ContainerID, mapping.HostID, mapping.Size))
	}
	return strings.Join(list, "_")
}

func recursiveChown(ctx context.Context, mounts []mount.Mount, uidMappings, gidMappings []IDMapping) error {
	uids := map[int]int{}
	gids := map[int]int{}
	return mount.WithTempMount(ctx, mounts, func(root string) error {
		return filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
			if err == nil {
				stat := info.Sys().(*syscall.Stat_t)
				u, g := int(stat.Uid), int(stat.Gid)
				if _, ok := uids[u]; !ok {
					uids[u] = getHostID(uidMappings, u)
				}
				if _, ok := gids[u]; !ok {
					gids[g] = getHostID(gidMappings, g)
				}
				if u == uids[u] && g == gids[g] {
					return nil
				}
				err = os.Lchown(name, uids[u], gids[g])
			}
			return err
		})
	})
}

func getHostID(mappings []IDMapping, containerID int) int {
	for _, mapping := range mappings {
		if delta := containerID - int(mapping.ContainerID); delta >= 0 && delta < int(mapping.Size) {
			return int(mapping.HostID) + delta
		}
	}

	return containerID
}
