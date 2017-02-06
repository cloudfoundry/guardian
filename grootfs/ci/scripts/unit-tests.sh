#!/bin/bash
set -e

sudo apt-get update
sudo apt-get install xfsprogs -y
source $(dirname $BASH_SOURCE)/test/utils.sh
sudo_mount_storage

dest_path=$(move_to_gopath grootfs)
cd $dest_path

echo "I AM groot" | grootsay

# containers/image gets angry when the home is wrong because it's trying to
# read $HOME/.docker
export HOME=/home/groot

args=$@
[ "$args" == "" ] && args="-r"
ginkgo -p -race -skipPackage integration $args
