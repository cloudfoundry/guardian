//go:build tools
// +build tools

package main

import (
	_ "code.cloudfoundry.org/cf-networking-helpers/healthchecker/cmd/healthchecker"
)

// This file imports packages that are used when running go generate, or used
// during the development process but not otherwise depended on by built code.
