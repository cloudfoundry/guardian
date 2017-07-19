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
        python \
        python-yaml \
        wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

##############################
# Install Bosh
RUN wget https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-2.0.28-linux-amd64 && \
    mv bosh-cli-* /usr/local/bin/bosh2 && \
    chmod +x /usr/local/bin/bosh2

################################
# Install CF
RUN wget "https://cli.run.pivotal.io/stable?release=debian64&version=6.28.0&source=github-rel" -O cf.deb && \
    dpkg -i cf.deb

###############################
# Setup the GOPATH
RUN mkdir -p /go && \
    mkdir -p /go/src/code.cloudfoundry.org/grootfs

################################
# Setup GO
ENV HOME /root
ENV GOPATH /go
ENV PATH /go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN mkdir -p $GOPATH
RUN \
  wget -qO- https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz | tar -C /usr/local -xzf -

################################
# Setup gaol
RUN wget https://github.com/contraband/gaol/releases/download/2016-8-22/gaol_linux -O /usr/bin/gaol && \
    chmod +x /usr/bin/gaol

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
RUN chown -R groot:groot /go

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
