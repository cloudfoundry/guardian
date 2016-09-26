#!/bin/bash
set -e

source $(dirname $BASH_SOURCE)/test/utils.sh
sudo_mount_btrfs

grootfs_path=$(move_to_gopath grootfs)
grootfs_bench_path=$(move_to_gopath grootfs-bench)

pushd $grootfs_path
  install_dependencies
  make
popd

cd $grootfs_bench_path
install_dependencies
make

echo "I AM GROOT" | grootsay

./grootfs-bench --gbin ../grootfs/grootfs --store /mnt/btrfs --concurrency 5
