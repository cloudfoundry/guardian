#!/bin/bash
set -e

env

source $(dirname $BASH_SOURCE)/test/utils.sh
sudo_mount_btrfs

grootfs_path=$(move_to_gopath grootfs)
grootfs_bench_path=$(move_to_gopath grootfs-bench)
grootfs_bench_performance_path="$grootfs_bench_path/performance"

pushd $grootfs_path
  install_dependencies
  make
popd

pushd $grootfs_bench_path
  install_dependencies
  make
popd

pushd /go/src/code.cloudfoundry.org/grootfs-bench/performance
  go build -o grootfs-performance-runner .

  perf_test_image="docker:///busybox"
  # warm the cache
  ../../grootfs/grootfs --store /mnt/btrfs create $perf_test_image my-warm-box &> /dev/null

  echo "RUNNING PERFORMANCE TESTS, STAND BY..." | grootsay

  ./grootfs-performance-runner \
    --benchBinPath ../grootfs-bench \
    [--gbin ../../grootfs/grootfs \
    --store /mnt/btrfs \
    --concurrency 5 \
    --image $perf_test_image \
    --nospin \
    --json $@]
popd
