#!/bin/bash
set -e

grootsay

grootfsPath=/go/src/code.cloudfoundry.org/grootfs
rmdir $grootfsPath
ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
cd $grootfsPath

# containers/image gets angry when the home is wrong because it's trying to
# read $HOME/.docker
export HOME=/home/groot

ginkgo -p -r -race $@
