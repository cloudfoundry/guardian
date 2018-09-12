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
	"errors"
	"syscall"

	"github.com/containerd/containerd/runtime/linux/runctypes"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// WithResources sets the provided resources for task updates
func WithResources(resources *specs.LinuxResources) UpdateTaskOpts {
	return func(ctx context.Context, client *Client, r *UpdateTaskInfo) error {
		r.Resources = resources
		return nil
	}
}

// WithNoNewKeyring causes tasks not to be created with a new keyring for secret storage.
// There is an upper limit on the number of keyrings in a linux system
func WithNoNewKeyring(ctx context.Context, c *Client, ti *TaskInfo) error {
	return updateTaskInfoCreateOptions(ti, func(opts *runctypes.CreateOptions) error {
		opts.NoNewKeyring = true
		return nil
	})
}

// WithCurrentUIDAndGID sets IoUid and IoGid to the calling user
// Note: This ensures that when fifo fds are created, they are chowned to the correct user
// This is important for when executing as a non-root user
func WithCurrentUIDAndGID(ctx context.Context, c *Client, ti *TaskInfo) error {
	return updateTaskInfoCreateOptions(ti, func(opts *runctypes.CreateOptions) error {
		opts.IoUid = uint32(syscall.Geteuid())
		opts.IoGid = uint32(syscall.Getegid())
		return nil
	})
}

func updateTaskInfoCreateOptions(taskInfo *TaskInfo, updateCreateOptions func(createOptions *runctypes.CreateOptions) error) error {
	if taskInfo.Options == nil {
		taskInfo.Options = &runctypes.CreateOptions{}
	}
	opts, ok := taskInfo.Options.(*runctypes.CreateOptions)
	if !ok {
		return errors.New("could not cast TaskInfo Options to CreateOptions")
	}

	return updateCreateOptions(opts)
}
