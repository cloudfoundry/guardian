#!/bin/bash

source $(dirname $BASH_SOURCE)/test/utils.sh
move_to_gopath

make go-vet
