#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh

trap unmount_storage EXIT

mount_storage

dest_path=$(move_to_gopath grootfs)
cd $dest_path

chmod +s /usr/bin/newuidmap
chmod +s /usr/bin/newgidmap

echo "I AM INTEGRATION: ${VOLUME_DRIVER} (${GROOTFS_TEST_UID}:${GROOTFS_TEST_GID})" | grootsay

umask 077

args=$@
[ "$args" == "" ] && args="-r integration"
ginkgo -p -race $args
