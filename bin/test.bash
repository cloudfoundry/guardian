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

# shellcheck disable=SC2068
# Double-quoting array expansion here causes ginkgo to fail
#runc
go run github.com/onsi/ginkgo/v2/ginkgo ${@}
#containerd
CONTAINERD_ENABLED=true go run github.com/onsi/ginkgo/v2/ginkgo ${@}
#containerd and cpu-throttling
CONTAINERD_ENABLED=true CPU_THROTTLING_ENABLED=true go run github.com/onsi/ginkgo/v2/ginkgo ${@}
