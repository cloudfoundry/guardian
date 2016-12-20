package config_test

import (
	"code.cloudfoundry.org/grootfs/commands/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validators", func() {
	Describe("ValidateBinary", func() {
		It("returns an error if can't find find the command in the $PATH", func() {
			err := config.ValidateBinary("ls-2")
			Expect(err.Error()).To(ContainSubstring(`exec: "ls-2": executable file not found in $PATH`))
		})
	})
})
