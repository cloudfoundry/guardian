#!/bin/bash
set -e -x


OUTPUT_PATH=$PWD/build-grootfs
GOPATH=$PWD/grootfs-release-master
VERSION=$(cat grootfs-release-version/number)

cd grootfs-release-master/src/code.cloudfoundry.org/grootfs

make
cp grootfs grootfs-${VERSION}
cp tardis tardis-${VERSION}

cp -r . $OUTPUT_PATH
