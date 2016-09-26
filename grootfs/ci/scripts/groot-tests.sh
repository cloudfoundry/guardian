#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath
install_dependencies
sudo_mount_btrfs

echo "I AM GROOT" | grootsay

# Setup drax
drax_path=$(compile_drax)
sudo_setup_drax $drax_path
cleanup_drax $drax_path

# containers/image gets angry when the home is wrong because it's trying to
# read $HOME/.docker
export HOME=/home/groot

args=$@
[ "$args" == "" ] && args="-r"
ginkgo -p -race $args
