#!/bin/bash
set -e

echo "I AM ROOT" | grootsay

ginkgo -mod vendor -p -race -r "$@"

