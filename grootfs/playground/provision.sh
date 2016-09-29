#!/bin/bash

set -e -x

apt-get -y update
apt-get -y clean

apt-get install -y curl git gcc make python-dev vim-nox jq btrfs-tools uidmap silversearcher-ag

wget -qO- https://storage.googleapis.com/golang/go1.7.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# Set up vim for golang development
if [ ! -d $HOME/.vim ]; then
  git clone https://github.com/luan/vimfiles.git $HOME/.vim
  $HOME/.vim/install --non-interactive
fi

# Set up bash-it
if [ ! -d $HOME/.bash_it ]; then
  git clone --depth=1 https://github.com/Bash-it/bash-it.git $HOME/.bash_it
  $HOME/.bash_it/install.sh -s
fi

# Set up git aliases
cat > $HOME/.bash_it/custom/git_config.bash <<EOF
#!/usr/bin/env bash
git config --global core.editor vim
git config --global core.pager "less -FXRS -x2"
git config --global alias.co checkout
git config --global alias.st status
git config --global alias.b branch
git config --global alias.plog "log --graph --abbrev-commit --decorate --date=relative --format=format:\'%C(bold blue)%h%C(reset) - %C(bold green)(%ar)%C(reset) %C(white)%s%C(reset) %C(dim white)- %an%C(reset)%C(bold yellow)%d%C(reset)' --all"
git config --global alias.lg "log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit --date=relative"
git config --global alias.flog = log --pretty=fuller --decorate
EOF

#Set up $GOPATH and add go executables to $PATH
cat > $HOME/.bash_it/custom/go_env.bash <<EOF
#!/usr/bin/env bash
export GOPATH=/home/ubuntu/go
export PATH=/home/ubuntu/go/bin:/usr/local/go/bin:$PATH
EOF
source $HOME/.bash_it/custom/go_env.bash

# Install awesome grootsay, I gotta say
go get github.com/fouralarmfire/grootsay

chown ubuntu:ubuntu $GOPATH

# Clone them all
go get code.cloudfoundry.org/grootfs
go get code.cloudfoundry.org/grootfs-bench
GROOT_PATH=$GOPATH/src/code.cloudfoundry.org/grootfs

# Install Management Dependendy tool
curl https://glide.sh/get | sh

# Build grootfs and move binaries around
pushd $GROOT_PATH
  make deps
  make
  cp {grootfs,drax} /usr/local/bin/
  cp hack/{quick-setup,cleanup-store} /usr/local/bin/
popd

# Configure btrfs environment
if [ ! -f /root/btrfs_volume ]; then
  quick-setup
fi

echo "Groot to go. Enjoy your day."
