#!/bin/bash

set -eu
set -o pipefail

export PATH="$PATH:$(dirname $RUNC_BINARY):$(dirname $CONTAINERD_BINARY)"
source "$CI_DIR/shared/helpers/filesystem-helpers.bash"
filesystem_permit_device_control
filesystem_create_loop_devices 256

# Set up AppArmor
if ! grep securityfs /proc/self/mounts > /dev/null 2>&1 ; then
    mount -t securityfs securityfs /sys/kernel/security
fi
apparmor_parser -r ../../jobs/garden/templates/config/garden-default

garden_rootfs_ext="${GARDEN_TEST_ROOTFS##*.}"
if [[ $garden_rootfs_ext != "tar" ]]; then
   garden_rootfs_tar="$(dirname $GARDEN_TEST_ROOTFS)/garden-rootfs.tar"
   tar -cf "$garden_rootfs_tar" -C $GARDEN_TEST_ROOTFS .
   export GARDEN_TEST_ROOTFS=$garden_rootfs_tar
fi

garden_fuse_ext="${GARDEN_FUSE_TEST_ROOTFS##*.}"
if [[ $garden_fuse_ext != "tar" ]]; then
   garden_fuse_tar="$(dirname $GARDEN_FUSE_TEST_ROOTFS)/garden-fuse.tar"
   tar -cf "$garden_fuse_tar" -C $GARDEN_FUSE_TEST_ROOTFS .
   export GARDEN_FUSE_TEST_ROOTFS=$garden_fuse_tar
fi


# shellcheck disable=SC2068
# Double-quoting array expansion here causes ginkgo to fail
#runc
go run github.com/onsi/ginkgo/v2/ginkgo ${@}
#containerd
CONTAINERD_ENABLED=true go run github.com/onsi/ginkgo/v2/ginkgo ${@}
#containerd and cpu-throttling
CONTAINERD_ENABLED=true CPU_THROTTLING_ENABLED=true go run github.com/onsi/ginkgo/v2/ginkgo ${@}
