package cloner_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func init() {
	if os.Args[1] == "untar" {
		fakeIDMapper := new(clonerfakes.FakeIDMapper)
		tarCloner := cloner.NewTarCloner(fakeIDMapper)

		logger := lagertest.NewTestLogger("test-tar-cloner-untar")
		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")

		toDir := os.Args[2]
		if path.Base(toDir) == "fail-to-untar" {
			os.Exit(1)
		}

		if err := tarCloner.Untar(logger, ctrlPipeR, toDir); err != nil {
			os.Exit(1)
		}

		os.Exit(0)
	}
}

var _ = Describe("TarCloner", func() {
	var (
		logger lager.Logger

		fromDir   string
		bundleDir string
		toDir     string

		fakeIDMapper *clonerfakes.FakeIDMapper
		tarCloner    *cloner.TarCloner
	)

	BeforeEach(func() {
		var err error

		fromDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(fromDir, "a_file"), []byte("hello-world"), 0600)).To(Succeed())

		bundleDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		toDir = path.Join(bundleDir, "rootfs")

		fakeIDMapper = new(clonerfakes.FakeIDMapper)
		tarCloner = cloner.NewTarCloner(fakeIDMapper)

		logger = lagertest.NewTestLogger("test-graph")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(fromDir)).To(Succeed())
		Expect(os.RemoveAll(bundleDir)).To(Succeed())
	})

	It("should have the image contents in the rootfs directory", func() {
		Expect(tarCloner.Clone(logger, groot.CloneSpec{
			FromDir: fromDir,
			ToDir:   toDir,
		})).To(Succeed())

		filePath := path.Join(toDir, "a_file")
		Expect(filePath).To(BeARegularFile())
		contents, err := ioutil.ReadFile(filePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("hello-world"))
	})

	Context("when using mapping users", func() {
		Describe("UIDMappings", func() {
			It("should use the uid provided", func() {
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
					UIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
					},
				})).To(Succeed())

				Expect(fakeIDMapper.MapUIDsCallCount()).To(Equal(1))
				_, mappings := fakeIDMapper.MapUIDsArgsForCall(0)

				Expect(mappings).To(Equal([]groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
				}))
			})

			Context("when it fails", func() {
				BeforeEach(func() {
					fakeIDMapper.MapUIDsReturns(errors.New("Boom!"))
				})

				It("should return an error", func() {
					Expect(tarCloner.Clone(logger, groot.CloneSpec{
						FromDir: fromDir,
						ToDir:   toDir,
						UIDMappings: []groot.IDMappingSpec{
							groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
						},
					})).To(MatchError(ContainSubstring("Boom!")))
				})
			})
		})

		Describe("GIDMappings", func() {
			It("should use the uid provided", func() {
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
					GIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
					},
				})).To(Succeed())

				Expect(fakeIDMapper.MapGIDsCallCount()).To(Equal(1))
				_, mappings := fakeIDMapper.MapGIDsArgsForCall(0)

				Expect(mappings).To(Equal([]groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
				}))
			})

			Context("when it fails", func() {
				BeforeEach(func() {
					fakeIDMapper.MapGIDsReturns(errors.New("Boom!"))
				})

				It("should return an error", func() {
					Expect(tarCloner.Clone(logger, groot.CloneSpec{
						FromDir: fromDir,
						ToDir:   toDir,
						GIDMappings: []groot.IDMappingSpec{
							groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
						},
					})).To(MatchError(ContainSubstring("Boom!")))
				})
			})
		})
	})

	Context("when the image path does not exist", func() {
		It("should return an error", func() {
			Expect(tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: "/does/not/exist",
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring("image path `/does/not/exist` was not found")),
			)
		})
	})

	Context("when the image contains files that can only be read by root", func() {
		It("should return an error", func() {
			Expect(ioutil.WriteFile(path.Join(fromDir, "a-file"), []byte("hello-world"), 0000)).To(Succeed())

			Expect(tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: fromDir,
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring(fmt.Sprintf("reading from `%s`", fromDir))),
			)
		})
	})

	Context("when untarring fails for reasons", func() {
		It("should return an error", func() {
			toDir := path.Join(bundleDir, "fail-to-untar")
			Expect(tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: fromDir,
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring(fmt.Sprintf("writing to `%s`", toDir))),
			)
		})
	})

	Context("when creating the target directory fails", func() {
		It("returns an error", func() {
			err := tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: fromDir,
				ToDir:   "/tmp/some-destination/bundles/1000",
			})

			Expect(err).To(MatchError(ContainSubstring("making destination directory")))
		})
	})
})
