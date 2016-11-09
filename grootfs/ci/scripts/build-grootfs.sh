#!/bin/bash
set -e -x


OUTPUT_PATH=$PWD/build-grootfs
GOPATH=$GOPATH:$PWD
VERSION=$(cat grootfs-release-version/number)

cd grootfs-release-master/src/code.cloudfoundry.org/grootfs

# Update CLI version.
grep grootfs.Version main.go | cut -d "=" -f2 | xargs -I % sed -i s:%:"${VERSION}":g main.go


git config --global user.email "grootfs-ci@localhost"
git config --global user.name "GrootFS CI Bot"

git add main.go
git commit -m "Bump version to ${VERSION}"

make
tar -czf grootfs.tgz grootfs
tar -czf drax.tgz drax

cp -r . $OUTPUT_PATH
