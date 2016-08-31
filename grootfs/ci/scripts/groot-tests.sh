#!/bin/bash
set -e

grootsay

source $(dirname $BASH_SOURCE)/test/utils.sh

# Setup groot package
move_to_gopath

# Setup BTRFS
sudo_mount_btrfs

# Setup drax
drax_path=$(compile_drax)
sudo_setup_drax $drax_path
cleanup_drax $drax_path

# containers/image gets angry when the home is wrong because it's trying to
# read $HOME/.docker
export HOME=/home/groot

ginkgo -p -r -race $@
