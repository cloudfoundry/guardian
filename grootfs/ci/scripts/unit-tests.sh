#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh

trap sudo_unmount_storage EXIT

sudo_mount_storage

echo "I AM groot" | grootsay

# containers/image gets angry when the home is wrong because it's trying to
# read $HOME/.docker
export HOME=/home/groot

args=$@
[ "$args" == "" ] && args="-r"
ginkgo -mod vendor -p -nodes 5 -race -skipPackage integration $args
