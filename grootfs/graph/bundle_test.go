package graph_test

import (
	"io/ioutil"
	"path"

	"code.cloudfoundry.org/grootfs/graph"
	"code.cloudfoundry.org/grootfs/groot"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bundle", func() {
	var (
		bundlePath string
		bundle     groot.Bundle
		err        error
	)

	BeforeEach(func() {
		bundlePath, err = ioutil.TempDir("", "bundle")
		Expect(err).NotTo(HaveOccurred())
		bundle = graph.NewBundle(bundlePath)
	})

	Describe("Path", func() {
		It("returns the bundle path", func() {
			Expect(bundle.Path()).To(Equal(bundlePath))
		})
	})

	Describe("RootFSPath", func() {
		It("returns the bundle rootfs path", func() {
			Expect(bundle.RootFSPath()).To(Equal(path.Join(bundlePath, "rootfs")))
		})
	})
})
