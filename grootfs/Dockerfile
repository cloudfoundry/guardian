FROM cloudfoundry/golang-ci
MAINTAINER https://github.com/cloudfoundry/grootfs

## Install uidmap utils
RUN apt-get install -y uidmap btrfs-tools

RUN dd bs=1024 count=100000 if=/dev/zero of=/btrfs_volume
RUN mkfs.btrfs /btrfs_volume

## Add groot user
RUN useradd -d /home/groot -m -U groot
RUN echo "groot ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers

RUN mkdir /go && \
	mkdir -p /go/src/code.cloudfoundry.org/grootfs && \
	chown -R groot:groot /go

## Run as groot
USER groot

## Env
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin
WORKDIR /go/src/code.cloudfoundry.org/grootfs

## Install Ginkgo
RUN go get github.com/onsi/ginkgo/ginkgo
RUN go install github.com/onsi/ginkgo/ginkgo

RUN go get github.com/fouralarmfire/grootsay
