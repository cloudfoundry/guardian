FROM cloudfoundry/golang-ci
MAINTAINER https://github.com/cloudfoundry/grootfs

## Install depedencies
RUN apt-get install -y uidmap btrfs-tools sudo jq

## Add groot user
RUN useradd -d /home/groot -m -U groot
RUN echo "groot ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

## Setup the GOPATH
RUN mkdir /go && \
	mkdir -p /go/src/code.cloudfoundry.org/grootfs && \
	chown -R groot:groot /go

## Make /root writable (e.g. /root/.docker/...)
RUN chmod 777 /root

## Run as groot
USER groot

## Env
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin
WORKDIR /go/src/code.cloudfoundry.org/grootfs

## Install stuff
RUN go get github.com/onsi/ginkgo/ginkgo
RUN go install github.com/onsi/ginkgo/ginkgo
RUN go get github.com/Masterminds/glide
RUN go get github.com/fouralarmfire/grootsay
