#!/bin/bash
set -e
set -x

install_apt_packages() {
  apt-get -y update
  apt-get -y clean
  apt-get install -y \
    gcc make \
    vim-nox \
    git silversearcher-ag curl jq \
    btrfs-tools uidmap \
    python-minimal
}

install_go() {
  source /root/.go_env || true
  if which go; then
    return
  fi

  wget -qO- https://storage.googleapis.com/golang/go1.7.linux-amd64.tar.gz | tar -C /usr/local -xzf -
}

configure_user() {
  user=${1:-$(whoami)}
  home=$(eval "echo ~$user")

  source $home/.go_env || true
  if which go; then
    return
  fi

  # Set up $GOPATH and add go executables to $PATH
  sudo -u $user cat > $home/.go_env <<EOF
export GOPATH=$home/go
export GOROOT=/usr/local/go
export PATH=\$PATH:\$GOPATH/bin:\$GOROOT/bin
EOF
  sudo -u $user cat >> $home/.bashrc <<EOF
source \$HOME/.go_env
EOF
  sudo -u $user mkdir -p $home/go
  sudo -u $user mkdir -p $home/go/bin
}

install_groot() {
  if which groot; then
    return
  fi

  source /root/.go_env
  # Install Management Dependendy tool
  go get github.com/Masterminds/glide

  # Install awesome grootsay, I gotta say
  go get github.com/fouralarmfire/grootsay

  # Clone them all
  [ ! -d $HOME/go/src/code.cloudfoundry.or/grootfs ] && go get code.cloudfoundry.org/grootfs
  go get code.cloudfoundry.org/grootfs-bench

  # Build grootfs and move binaries around
  pushd $HOME/go/src/code.cloudfoundry.org/grootfs
    make deps
    make
    cp {grootfs,drax} /usr/local/bin
    chmod u+s /usr/local/bin/drax
    cp hack/{quick-setup,cleanup-store} /usr/local/bin
  popd
}

setup_btrfs() {
  if [ -d /var/lib/grootfs ]; then
    return
  fi

  # Configure btrfs environment
  quick-setup
}

install_apt_packages
install_go
configure_user "root"
configure_user "ubuntu"
install_groot
setup_btrfs
echo "Groot to go. Enjoy your day."
