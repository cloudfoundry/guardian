#!/bin/bash
set -e

grootsay

grootfsPath=/go/src/code.cloudfoundry.org/grootfs
rmdir $grootfsPath
ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
cd $grootfsPath

ginkgo -p -r -race $@
