#!/usr/bin/env bash
set -eo pipefail

source "$(dirname "$BASH_SOURCE")/test/utils.sh"

trap unmount_storage EXIT

mount_storage

make
make prefix=/usr/bin install

chmod +s /usr/bin/newuidmap
chmod +s /usr/bin/newgidmap

echo "I AM INTEGRATION: ${VOLUME_DRIVER} (${GROOTFS_TEST_UID}:${GROOTFS_TEST_GID})" | grootsay

umask 077

args=$@
[ "$args" == "" ] && args="-r integration"
ginkgo -mod vendor -p -nodes 5 -race $args
