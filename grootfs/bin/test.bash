#!/bin/bash

set -eu
set -o pipefail

source "$CI_DIR/shared/helpers/filesystem-helpers.bash"
filesystem_permit_device_control
filesystem_create_loop_devices 256
trap filesystem_unmount_storage EXIT
filesystem_mount_storage

# user without root permission in tas-runtime-build-* images
export GROOTFS_USER="nonroot"

: "DOCKER_REGISTRY_USERNAME: ${DOCKER_REGISTRY_USERNAME:?Need to set DOCKER_REGISTRY_USERNAME}"
: "DOCKER_REGISTRY_PASSWORD: ${DOCKER_REGISTRY_PASSWORD:?Need to set DOCKER_REGISTRY_PASSWORD}"

# shellcheck disable=SC2068
# Double-quoting array expansion here causes ginkgo to fail
GROOTFS_TEST_UID=0 GROOTFS_TEST_GID=0 go run github.com/onsi/ginkgo/v2/ginkgo ${@}
