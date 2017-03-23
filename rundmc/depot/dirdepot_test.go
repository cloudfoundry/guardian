package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/depot"
	fakes "code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Depot", func() {
	var (
		depotDir    string
		bundleSaver *fakes.FakeBundleSaver
		dirdepot    *depot.DirectoryDepot
		logger      lager.Logger
		bndle       goci.Bndl
		rootFSPath  string
	)

	BeforeEach(func() {
		var err error

		depotDir, err = ioutil.TempDir("", "depot-test")
		Expect(err).NotTo(HaveOccurred())

		rootFSPath, err = ioutil.TempDir("", "depot-test")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(rootFSPath, "etc"), 0700)).To(Succeed())
		Expect(touchFile(filepath.Join(rootFSPath, "etc", "hosts"))).To(Succeed())
		Expect(touchFile(filepath.Join(rootFSPath, "etc", "resolv.conf"))).To(Succeed())

		bndle = goci.Bndl{Spec: specs.Spec{Root: specs.Root{Path: rootFSPath}}}

		logger = lagertest.NewTestLogger("test")

		bundleSaver = new(fakes.FakeBundleSaver)
		dirdepot = depot.New(depotDir, bundleSaver)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depotDir)).To(Succeed())
		Expect(os.RemoveAll(rootFSPath)).To(Succeed())
	})

	Describe("lookup", func() {
		Context("when a subdirectory with the given name does not exist", func() {
			It("returns an ErrDoesNotExist", func() {
				_, err := dirdepot.Lookup(logger, "potato")
				Expect(err).To(MatchError(depot.ErrDoesNotExist))
			})
		})

		Context("when a subdirectory with the given name exists", func() {
			It("returns the absolute path of the directory", func() {
				os.Mkdir(filepath.Join(depotDir, "potato"), 0700)
				Expect(dirdepot.Lookup(logger, "potato")).To(Equal(filepath.Join(depotDir, "potato")))
			})
		})
	})

	Describe("create", func() {
		It("should create a directory", func() {
			Expect(dirdepot.Create(logger, "aardvaark", bndle)).To(Succeed())
			Expect(filepath.Join(depotDir, "aardvaark")).To(BeADirectory())
		})

		It("creates a hosts file", func() {
			Expect(dirdepot.Create(logger, "aardvaark", bndle)).To(Succeed())
			Expect(filepath.Join(depotDir, "aardvaark", "hosts")).To(BeAnExistingFile())
		})

		It("creates a resolv.conf file", func() {
			Expect(dirdepot.Create(logger, "aardvaark", bndle)).To(Succeed())
			Expect(filepath.Join(depotDir, "aardvaark", "resolv.conf")).To(BeAnExistingFile())
		})

		It("should serialize the container config to the directory with mounts for hosts and resolv.conf", func() {
			Expect(dirdepot.Create(logger, "aardvaark", bndle)).To(Succeed())
			Expect(bundleSaver.SaveCallCount()).To(Equal(1))
			actualBundle, actualPath := bundleSaver.SaveArgsForCall(0)
			Expect(actualPath).To(Equal(path.Join(depotDir, "aardvaark")))
			Expect(actualBundle.Mounts()).To(ConsistOf(
				specs.Mount{
					Destination: "/etc/hosts",
					Source:      filepath.Join(depotDir, "aardvaark", "hosts"),
					Type:        "bind",
					Options:     []string{"bind"},
				},
				specs.Mount{
					Destination: "/etc/resolv.conf",
					Source:      filepath.Join(depotDir, "aardvaark", "resolv.conf"),
					Type:        "bind",
					Options:     []string{"bind"},
				},
			))
		})

		Context("when /etc/hosts does not exist in the container rootFS", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(rootFSPath, "etc", "hosts"))).To(Succeed())
			})

			It("should serialize the container config to the directory without a mount for /etc/hosts", func() {
				Expect(dirdepot.Create(logger, "aardvaark", bndle)).To(Succeed())
				Expect(bundleSaver.SaveCallCount()).To(Equal(1))
				actualBundle, _ := bundleSaver.SaveArgsForCall(0)
				Expect(actualBundle.Mounts()).To(ConsistOf(
					specs.Mount{
						Destination: "/etc/resolv.conf",
						Source:      filepath.Join(depotDir, "aardvaark", "resolv.conf"),
						Type:        "bind",
						Options:     []string{"bind"},
					},
				))
			})
		})

		Context("when /etc/resolv.conf does not exist in the container rootFS", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(rootFSPath, "etc", "resolv.conf"))).To(Succeed())
			})

			It("should serialize the container config to the directory without a mount for /etc/resolv.conf", func() {
				Expect(dirdepot.Create(logger, "aardvaark", bndle)).To(Succeed())
				Expect(bundleSaver.SaveCallCount()).To(Equal(1))
				actualBundle, _ := bundleSaver.SaveArgsForCall(0)
				Expect(actualBundle.Mounts()).To(ConsistOf(
					specs.Mount{
						Destination: "/etc/hosts",
						Source:      filepath.Join(depotDir, "aardvaark", "hosts"),
						Type:        "bind",
						Options:     []string{"bind"},
					},
				))
			})
		})

		Context("when neither /etc/resolv.conf nor /etc/hosts exist in the container rootFS", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(rootFSPath, "etc", "resolv.conf"))).To(Succeed())
				Expect(os.Remove(filepath.Join(rootFSPath, "etc", "hosts"))).To(Succeed())
			})

			It("should serialize the container config to the directory without any mounts", func() {
				Expect(dirdepot.Create(logger, "aardvaark", bndle)).To(Succeed())
				Expect(bundleSaver.SaveCallCount()).To(Equal(1))
				actualBundle, _ := bundleSaver.SaveArgsForCall(0)
				Expect(actualBundle.Mounts()).To(BeEmpty())
			})
		})

		It("destroys the container directory if creation fails", func() {
			bundleSaver.SaveReturns(errors.New("didn't work"))
			Expect(dirdepot.Create(logger, "aardvaark", bndle)).NotTo(Succeed())
			Expect(filepath.Join(depotDir, "aardvaark")).NotTo(BeADirectory())
		})
	})

	Describe("destroy", func() {
		It("should destroy the container directory", func() {
			Expect(os.MkdirAll(filepath.Join(depotDir, "potato"), 0755)).To(Succeed())
			Expect(dirdepot.Destroy(logger, "potato")).To(Succeed())
			Expect(filepath.Join(depotDir, "potato")).NotTo(BeAnExistingFile())
		})

		Context("when the container directory does not exist", func() {
			It("does not error (i.e. the method is idempotent)", func() {
				Expect(dirdepot.Destroy(logger, "potato")).To(Succeed())
			})
		})
	})

	Describe("handles", func() {
		Context("when handles exist", func() {
			BeforeEach(func() {
				Expect(dirdepot.Create(logger, "banana", bndle)).To(Succeed())
				Expect(dirdepot.Create(logger, "banana2", bndle)).To(Succeed())
			})

			It("should return the handles", func() {
				Expect(dirdepot.Handles()).To(ConsistOf("banana", "banana2"))
			})
		})

		Context("when no handles exist", func() {
			It("should return an empty list", func() {
				Expect(dirdepot.Handles()).To(BeEmpty())
			})
		})

		Context("when the depot directory does not exist", func() {
			var invalidDepot *depot.DirectoryDepot

			BeforeEach(func() {
				invalidDepot = depot.New("rubbish", bundleSaver)
			})

			It("should return the handles", func() {
				_, err := invalidDepot.Handles()
				Expect(err).To(MatchError("invalid depot directory rubbish: open rubbish: no such file or directory"))
			})
		})
	})
})

func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return file.Close()
}
