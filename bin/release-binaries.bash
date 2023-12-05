#!/bin/bash

set -eu
set -o pipefail

function run() {
    local arch os output
    arch="${1:?Please provide an architecture}"
    os="${2:?Please provide an OS}"
    version="${3:?Please provide a version}"
    output="${4:?Please provide an output directory}"

    mkdir -p $PWD/linux/bin
    mkdir -p $PWD/linux/sbin
    local tmpDir="linux"

    cp -aL ${TAR_BINARY} "linux/bin/tar"
    cp -aL ${IPTABLES_BINARY} "linux/sbin/iptables"
    cp -aL ${IPTABLES_RESTORE_BINARY} "linux/sbin/iptables-restore"
    cp -aL ${RUNC_BINARY} "linux/bin/runc"
    cp -aL ${DADOO_BINARY} "linux/bin/dadoo"
    cp -aL ${NSTAR_BINARY} "linux/bin/nstar"
    cp -aL ${INIT_BINARY} "linux/bin/init"
    cp -aL ${GROOTFS_BINARY} "linux/bin/grootfs"
    cp -aL ${GROOTFS_TARDIS_BINARY} "linux/bin/tardis"
    cp -aL ${IDMAPPER_NEWUIDMAP_BINARY} "linux/bin/newuidmap"
    cp -aL ${IDMAPPER_NEWGIDMAP_BINARY} "linux/bin/newgidmap"
    cp -aL ${IDMAPPER_MAXIMUS_BINARY} "linux/bin/maximus"

  go install github.com/go-bindata/go-bindata/go-bindata@latest
  go-bindata -nomemcopy -pkg bindata -o bindata/bindata.go linux/...

    GOARCH="${arch}" GOOS="${os}" \
        go build \
        -tags daemon \
        -o "${output}/gdn-${os}-${arch}" \
        -ldflags "-X main.Version=${version} -extldflags=-static" \
        ./cmd/gdn
}

run "$@"
