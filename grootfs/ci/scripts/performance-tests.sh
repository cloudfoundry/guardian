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

perf_test_image="docker:///busybox"
# warm the cache
../grootfs/grootfs --store /mnt/btrfs create $perf_test_image my-warm-box > /dev/null

echo "RUNNING PERFORMANCE TESTS, STAND BY..." | grootsay

./grootfs-bench --gbin ../grootfs/grootfs --store /mnt/btrfs --concurrency 5 --image $perf_test_image --nospin $@
