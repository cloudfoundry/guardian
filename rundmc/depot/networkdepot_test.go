package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	fakes "code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NetworkDepot", func() {
	var (
		dir                    string
		bindMountSourceCreator *fakes.FakeBindMountSourceCreator
		rootfsFileCreator      *fakes.FakeRootfsFileCreator
		logger                 lager.Logger
		networkDepot           depot.NetworkDepot
	)

	BeforeEach(func() {
		var err error
		dir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		logger = lagertest.NewTestLogger("test")
		rootfsFileCreator = new(fakes.FakeRootfsFileCreator)
		bindMountSourceCreator = new(fakes.FakeBindMountSourceCreator)
		networkDepot = depot.NewNetworkDepot(dir, rootfsFileCreator, bindMountSourceCreator)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(dir)).To(Succeed())
	})

	Describe("SetupBindMounts", func() {
		var (
			bindMounts []garden.BindMount
			setupErr   error
		)

		BeforeEach(func() {
			bindMountSourceCreator.CreateReturns([]garden.BindMount{{SrcPath: "src"}}, nil)
		})

		JustBeforeEach(func() {
			bindMounts, setupErr = networkDepot.SetupBindMounts(logger, "my-container", true, "/path/to/rootfs")
		})

		It("succeeds", func() {
			Expect(setupErr).NotTo(HaveOccurred())
		})

		It("creates the rootfs files", func() {
			Expect(rootfsFileCreator.CreateFilesCallCount()).To(Equal(1))
			actualRootfsPath, actualFiles := rootfsFileCreator.CreateFilesArgsForCall(0)
			Expect(actualRootfsPath).To(Equal("/path/to/rootfs"))
			Expect(actualFiles).To(ConsistOf("/etc/hosts", "/etc/resolv.conf"))
		})

		Context("when creating the rootfs files fails", func() {
			BeforeEach(func() {
				rootfsFileCreator.CreateFilesReturns(errors.New("failed"))
			})

			It("returns the error", func() {
				Expect(setupErr).To(MatchError("failed"))
			})
		})

		It("creates container directory", func() {
			containerDir := filepath.Join(dir, "my-container")
			Expect(containerDir).To(BeADirectory())
		})

		It("creates the network bind mounts", func() {
			Expect(bindMountSourceCreator.CreateCallCount()).To(Equal(1))
			actualContainerDir, actualPrivileged := bindMountSourceCreator.CreateArgsForCall(0)
			Expect(actualContainerDir).To(Equal(filepath.Join(dir, "my-container")))
			Expect(actualPrivileged).To(BeFalse())
			Expect(bindMounts).To(ConsistOf(garden.BindMount{SrcPath: "src"}))
		})

		Context("when creating the network bind mounts fails", func() {
			BeforeEach(func() {
				bindMountSourceCreator.CreateReturns(nil, errors.New("failed"))
			})

			It("returns the error", func() {
				Expect(setupErr).To(MatchError("failed"))
			})

			It("deletes the container depot directory", func() {
				containerDir := filepath.Join(dir, "my-container")
				Expect(containerDir).NotTo(BeADirectory())
			})
		})
	})

	Describe("Destroy", func() {
		It("deletes the depot directory", func() {
			_, err := networkDepot.SetupBindMounts(logger, "my-container", true, "/path/to/rootfs")
			Expect(err).NotTo(HaveOccurred())

			Expect(networkDepot.Destroy(logger, "my-container")).To(Succeed())
			containerDir := filepath.Join(dir, "my-container")
			Expect(containerDir).NotTo(BeADirectory())
		})
	})
})
