#!/bin/bash

source $(dirname $BASH_SOURCE)/test/utils.sh
grootfs_path=$(move_to_gopath grootfs)

cd $grootfs_path
make
make windows
