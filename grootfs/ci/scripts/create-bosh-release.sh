#!/bin/bash
# vim: set ft=sh

set -e -x

VERSION=$(cat ./grootfs-release-version-2/number)
if [ -z "$VERSION" ]; then
  echo "missing version number"
  exit 1
fi

(
  cd grootfs-release-git-repo/
  bosh -n create release --with-tarball --version "${VERSION}"
)

mkdir -p bosh-release
mv grootfs-release-git-repo/dev_releases/grootfs/*.tgz bosh-release/grootfs-${VERSION}.tgz
