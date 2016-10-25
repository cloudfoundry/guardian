#!/bin/bash
set -e

move_to_gopath() {
  thing_i_want_moved=$1
  dest_path=/go/src/code.cloudfoundry.org/${thing_i_want_moved}

  # remove the original package path
  [ -d $dest_path ] && rmdir $dest_path

  # link the uploaded source (from build) to the GOPATH
  ln -s $PWD/src/code.cloudfoundry.org/${thing_i_want_moved} $dest_path

  # because the uploaded source is owned by the user that runs fly, we need
  # to chown
  pushd $dest_path
    sudo chown -R groot:groot .
  popd

  echo $dest_path
}
dest_path=$(move_to_gopath idmapper)
cd $dest_path


go get github.com/onsi/gomega

echo "I AM ROOT" | grootsay

args=$@
[ "$args" == "" ] && args="-r integration/root"
ginkgo -p -race $args

