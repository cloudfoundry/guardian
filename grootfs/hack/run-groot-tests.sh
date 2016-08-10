#!/bin/bash
set -e

hack_path=$(dirname $BASH_SOURCE)
source $hack_path/test/test-utils.sh

show_groot_banner

grootfsPath=/go/src/code.cloudfoundry.org/grootfs
rmdir $grootfsPath
ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
cd $grootfsPath

ginkgo -p -r -race $@
