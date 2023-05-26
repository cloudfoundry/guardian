#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh

trap sudo_unmount_storage EXIT

sudo_mount_storage

echo "I AM groot"

# containers/image gets angry when the home is wrong because it's trying to
# read $HOME/.docker
export HOME=/home/groot

args=$@
[ "$args" == "" ] && args="-r"
go mod vendor # i don't know why this is necessary, but in CI this is required for the
              # go run to work. when done locally this isn't necessary. no files end up changing
              # in the repo. 
go run github.com/onsi/ginkgo/v2/ginkgo -p -nodes 5 -race -skip-package integration $args
