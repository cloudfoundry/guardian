#!/bin/bash
set -e -x


OUTPUT_PATH=$PWD/build-grootfs
GOPATH=$GOPATH:$PWD
VERSION=$(cat grootfs-version/number)

cd src/code.cloudfoundry.org/grootfs

# Update CLI version.
grep grootfs.Version main.go | cut -d "=" -f2 | xargs -I % sed -i s:%:"${VERSION}":g main.go


git config --global user.email "grootfs-ci@localhost"
git config --global user.name "GrootFS CI Bot"

git add main.go
git commit -m "Bump version to ${VERSION}"

GOOS=linux go build -o grootfs .
tar -czf grootfs.tgz grootfs

cp -r . $OUTPUT_PATH
