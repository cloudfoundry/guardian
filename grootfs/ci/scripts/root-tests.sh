#!/bin/bash
set -e

grootsay I AM ROOT

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath
mount_btrfs

ginkgo -p -r -race integration/root
