#!/bin/bash
set -e

grootsay

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath
sudo_mount_btrfs

# containers/image gets angry when the home is wrong because it's trying to
# read $HOME/.docker
export HOME=/home/groot

ginkgo -p -r -race $@
