#!/bin/bash
set -e

echo "I AM ROOT" | grootsay

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath
mount_btrfs

args=$@
[ "$args" == "" ] && args="-r integration/root"
ginkgo -p -race $args
