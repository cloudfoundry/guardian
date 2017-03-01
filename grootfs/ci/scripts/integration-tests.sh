#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh
mount_storage

dest_path=$(move_to_gopath grootfs)
cd $dest_path

echo "I AM INTEGRATION: ${VOLUME_DRIVER} (${GROOTFS_TEST_UID}:${GROOTFS_TEST_GID})" | grootsay

args=$@
[ "$args" == "" ] && args="-r integration"
ginkgo -p -race $args
