FROM cloudfoundry/golang-ci
MAINTAINER https://github.com/cloudfoundry/grootfs

## Install uidmap utils
RUN apt-get install -y uidmap

## Add groot user
RUN useradd -U groot

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
