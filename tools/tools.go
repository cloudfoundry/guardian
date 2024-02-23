//go:build tools
// +build tools

package main

import (
	_ "github.com/containerd/containerd/v2/cmd/containerd"
	_ "github.com/containerd/containerd/v2/cmd/containerd-shim-runc-v2"
	_ "github.com/containerd/containerd/v2/cmd/ctr"
	_ "github.com/opencontainers/runc"

	_ "github.com/maxbrunsfeld/counterfeiter/v6"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
)

// This file imports packages that are used when running go generate, or used
// during the development process but not otherwise depended on by built code.
