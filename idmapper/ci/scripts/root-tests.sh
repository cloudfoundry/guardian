#!/bin/bash
set -e

ginkgo -mod vendor -p -race -r "$@"

