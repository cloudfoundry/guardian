#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh
mount_btrfs

dest_path=$(move_to_gopath grootfs)
cd $dest_path
install_dependencies

echo "I AM ROOT" | grootsay

args=$@
[ "$args" == "" ] && args="-r integration/root"
ginkgo -p -race $args
