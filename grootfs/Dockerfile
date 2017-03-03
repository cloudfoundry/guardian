FROM ubuntu:16.04
MAINTAINER https://github.com/cloudfoundry/garden-dockerfiles

################################
# Install system packages
RUN apt-get update && \
    apt-get -y install \
        btrfs-tools \
        xfsprogs \
        build-essential \
        curl \
        git \
        jq \
        vim \
        netcat \
        sudo \
        uidmap \
        unzip \
        wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

##############################
# Install Bosh
RUN wget https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-2.0.1-linux-amd64 && \
    mv bosh-cli-* /usr/local/bin/bosh2 && \
    chmod +x /usr/local/bin/bosh2

################################
# Setup GO
ENV HOME /root
ENV GOPATH /root/go
ENV PATH /root/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN mkdir -p $GOPATH
RUN \
  wget -qO- https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz | tar -C /usr/local -xzf -

################################
# Install Go packages
RUN go get github.com/onsi/ginkgo/ginkgo
RUN go install github.com/onsi/ginkgo/ginkgo
RUN go get github.com/onsi/gomega
RUN go get github.com/Masterminds/glide
RUN go get github.com/fouralarmfire/grootsay

################################
# Add groot user
RUN useradd -d /home/groot -m -U groot
RUN echo "groot ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

###############################
# Setup the GOPATH
RUN mkdir -p /go && \
    mkdir -p /go/src/code.cloudfoundry.org/grootfs && \
    chown -R groot:groot /go

################################
# Make /root writable (e.g. /root/.docker/...)
RUN chmod 777 /root

###############################
# Run as groot
USER groot

###############################
# Env
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin
WORKDIR /go/src/code.cloudfoundry.org/grootfs
