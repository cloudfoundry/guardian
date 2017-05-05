#!/bin/bash
set -e -x


OUTPUT_PATH=$PWD/build-grootfs
GOPATH=$PWD/grootfs-release-master
VERSION=$(cat grootfs-release-version/number)

cd grootfs-release-master/src/code.cloudfoundry.org/grootfs

make
tar -czf grootfs-${VERSION}.tgz grootfs
tar -czf drax-${VERSION}.tgz drax
cp drax drax-${VERSION}
cp grootfs grootfs-${VERSION}

cp -r . $OUTPUT_PATH
