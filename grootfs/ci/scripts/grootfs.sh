#!/bin/bash
set -e

cat $(dirname $0)/../misc/groot.ascii
echo
echo "I AM "$(whoami)
echo

grootfsPath=/go/src/code.cloudfoundry.org/grootfs
rmdir $grootfsPath
ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
cd $grootfsPath

make test
