package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	nstarBin string
	tarBin   string
)

func TestNstar(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		var err error

		nstarBin, err = gexec.Build("nstar_windows.go")
		Expect(err).ToNot(HaveOccurred())

		tarBin, err = gexec.Build("fakes/tar.go")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	RunSpecs(t, "Nstar Suite")
}
