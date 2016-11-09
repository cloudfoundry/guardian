package store_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("RootfsConfigurer", func() {
	var (
		configurer *store.RootFSConfigurer
		baseImage  specsv1.Image
		rootFSPath string
	)

	BeforeEach(func() {
		var err error
		rootFSPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		baseImage = specsv1.Image{
			Config: specsv1.ImageConfig{
				Volumes: map[string]struct{}{
					"/volume-a/nested/dir": struct{}{},
					"/volume-b":            struct{}{},
				},
			},
		}

		configurer = store.NewRootFSConfigurer()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootFSPath)).To(Succeed())
	})

	Describe("Configurer", func() {
		It("creates the volumes", func() {
			Expect(configurer.Configure(rootFSPath, baseImage)).To(Succeed())
			Expect(filepath.Join(rootFSPath, "volume-a")).To(BeADirectory())
			Expect(filepath.Join(rootFSPath, "volume-b")).To(BeADirectory())
		})

		Context("when rootfs path does not have permissions", func() {
			BeforeEach(func() {
				Expect(os.Chmod(rootFSPath, 0000)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(configurer.Configure(rootFSPath, baseImage)).To(MatchError(ContainSubstring("making volume")))
			})
		})

		Context("when there is a file with the same volume path", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(rootFSPath, "volume-b"), []byte(""), 0755)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(configurer.Configure(rootFSPath, baseImage)).To(MatchError(ContainSubstring("a file with the requested volume path already exists")))
			})
		})

		Context("when the rootfs does not exist", func() {
			BeforeEach(func() {
				rootFSPath = "/path/to/rootfs"
			})

			It("returns an error", func() {
				Expect(configurer.Configure(rootFSPath, baseImage)).To(MatchError(ContainSubstring("rootfs path does not exist")))
			})
		})

		Context("when the volume starts with `..`", func() {
			BeforeEach(func() {
				baseImage.Config.Volumes["../volume-c"] = struct{}{}
			})

			It("returns an error", func() {
				Expect(configurer.Configure(rootFSPath, baseImage)).To(MatchError(ContainSubstring("volume path is outside of the rootfs")))
			})
		})

		Context("when the volume contains `..`", func() {
			BeforeEach(func() {
				baseImage.Config.Volumes["/../volume-c"] = struct{}{}
			})

			It("creates the volume", func() {
				Expect(configurer.Configure(rootFSPath, baseImage)).To(Succeed())
				Expect(filepath.Join(rootFSPath, "volume-c")).To(BeADirectory())
			})
		})
	})
})
