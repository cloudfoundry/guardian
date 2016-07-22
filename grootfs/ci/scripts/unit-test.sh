#!/bin/bash -x

export GOPATH=/root/go:$PWD
echo "I AM GROOT"

ginkgo -r
