#!/bin/bash

cat $(dirname $0)/../misc/groot.ascii
echo
echo "I AM GROOT"
echo

export GOPATH=/root/go:$PWD
ginkgo -r -p .
