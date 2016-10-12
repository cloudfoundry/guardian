#!/bin/bash
set -e

env

source $(dirname $BASH_SOURCE)/test/utils.sh
sudo_mount_btrfs

grootfs_path=$(move_to_gopath grootfs)
grootfs_bench_path=$(move_to_gopath grootfs-bench)
grootfs_bench_reporter_path="$grootfs_bench_path/reporter"

pushd $grootfs_path
  EVENT_TITLE=$(git log --oneline -n 1)
  EVENT_MESSAGE=$(git log -1 --pretty=%B)
  install_dependencies
  make
  sudo_setup_drax ./drax
popd

pushd $grootfs_bench_path
  install_dependencies
  make
popd

pushd /go/src/code.cloudfoundry.org/grootfs-bench/reporter
  go build -o grootfs-reporter .

  perf_test_image="docker:///busybox"
  # warm the cache
  ../../grootfs/grootfs --store /mnt/btrfs create $perf_test_image my-warm-box &> /dev/null

  echo "RUNNING PERFORMANCE TESTS, STAND BY..." | grootsay

  ./grootfs-reporter \
    --mode event \
    --eventTitle "$EVENT_TITLE" \
    --eventMessage "$EVENT_MESSAGE"

  ./grootfs-reporter \
    --mode runner \
    --benchBinPath ../grootfs-bench \
    [--gbin ../../grootfs/grootfs \
    --store /mnt/btrfs \
    --concurrency 5 \
    --with-quota \
    --image $perf_test_image \
    --nospin \
    --json $@]
popd
