#!/bin/bash
set -e

cat $(dirname $0)/../misc/groot.ascii
echo
echo "I AM GROOT"
echo

grootfsPath=/go/src/code.cloudfoundry.org/grootfs
rmdir $grootfsPath
ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
cd $grootfsPath

su groot -c "PATH=$PATH ginkgo -p -r $@"
ginkgo -p -r integration/root
