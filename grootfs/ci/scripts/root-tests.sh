#!/bin/bash
set -e

grootsay

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath

ginkgo -p -r -race integration/root
