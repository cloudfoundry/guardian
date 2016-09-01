#!/bin/bash
set -e

grootsay I AM ROOT

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath
mount_btrfs

args=$@
[ "$args" == "" ] && args="integration/root"
ginkgo -p -r -race $args
