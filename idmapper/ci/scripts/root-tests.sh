#!/bin/bash
set -e

go install github.com/onsi/ginkgo/ginkgo@latest
ginkgo -mod vendor -p -race -r "$@"

