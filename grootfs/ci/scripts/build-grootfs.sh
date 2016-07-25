#!/bin/bash
set -e -x

BUILD_PATH=$PWD

grootfsPath=/go/src/code.cloudfoundry.org/grootfs
rmdir $grootfsPath
ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
cd $grootfsPath

GOOS=linux go build -o grootfs .
tar -cfz grootfs.tgz grootfs
mv groofs.tgz $BUILD_PATH/build-grootfs/
