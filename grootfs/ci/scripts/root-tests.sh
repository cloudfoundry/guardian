#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath
install_dependencies
mount_btrfs

echo "I AM ROOT" | grootsay

args=$@
[ "$args" == "" ] && args="-r integration/root"
ginkgo -p -race $args
