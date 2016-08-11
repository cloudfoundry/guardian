function show_groot_banner {
  cat $(dirname $BASH_SOURCE)/groot.ascii
  echo
  echo "I AM $(whoami)"
  echo
}

function move_to_gopath {
  grootfsPath=/go/src/code.cloudfoundry.org/grootfs
  rmdir $grootfsPath
  ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
  cd $grootfsPath
}
