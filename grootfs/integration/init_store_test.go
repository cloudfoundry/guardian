package integration_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store/manager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Init Store", func() {
	const (
		rootUID = 0
		rootGID = 0
	)

	var (
		storePath string
		runner    runner.Runner
		spec      manager.InitSpec
	)

	BeforeEach(func() {
		spec = manager.InitSpec{}
		integration.SkipIfNonRoot(GrootfsTestUid)
		storePath = filepath.Join(StorePath, fmt.Sprintf("init-store-%d", rand.Int()))
		runner = Runner.WithStore(storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath))
	})

	It("returns a newly created store path", func() {
		Expect(runner.InitStore(spec)).To(Succeed())

		Expect(runner.StorePath).To(BeADirectory())

		stat, err := os.Stat(runner.StorePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))
	})

	It("sets the ownership to the caller user", func() {
		Expect(runner.InitStore(spec)).To(Succeed())

		Expect(runner.StorePath).To(BeADirectory())

		stat, err := os.Stat(runner.StorePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(rootUID)))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(rootGID)))
	})

	It("can create images with the same mappings", func() {
		Expect(runner.InitStore(spec)).To(Succeed())

		_, err := runner.Create(groot.CreateSpec{
			ID:        "random-id",
			BaseImage: "docker:///cfgarden/empty",
		})

		Expect(err).ToNot(HaveOccurred())
	})

	Context("when the store is used to create a image with different mappings", func() {
		It("fails", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			_, err := runner.Create(groot.CreateSpec{
				ID:        "random-id",
				BaseImage: "docker:///cfgarden/empty",
				UIDMappings: []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
				GIDMappings: []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
			})

			Expect(err).To(MatchError("store already initialized with a different mapping"))
		})
	})

	Context("when id mappings are provided", func() {
		BeforeEach(func() {
			spec.UIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}
			spec.GIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}
		})

		It("sets the ownership to the user mapped as root", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			Expect(runner.StorePath).To(BeADirectory())

			stat, err := os.Stat(runner.StorePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
		})

		It("can create images with the same mappings", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			_, err := runner.Create(groot.CreateSpec{
				ID:        "random-id",
				BaseImage: "docker:///cfgarden/empty",
				UIDMappings: []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
				GIDMappings: []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
			})

			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the store is used to create a image with different mappings", func() {
			It("fails", func() {
				Expect(runner.InitStore(spec)).To(Succeed())

				_, err := runner.Create(groot.CreateSpec{
					ID:        "random-id",
					BaseImage: "docker:///cfgarden/empty",
					UIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
					},
					GIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
					},
				})

				Expect(err).To(MatchError("store already initialized with a different mapping"))
			})
		})
	})

	Context("when the given driver does not match the mounted path", func() {
		BeforeEach(func() {
			runner = Runner.WithStore("/mnt/ext4/grootfs")
		})

		It("returns an error", func() {
			err := runner.InitStore(spec)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("Store path filesystem (/mnt/ext4) is incompatible with requested driver"))
			Expect(runner.StorePath).ToNot(BeADirectory())
		})
	})

	Context("when the given store path is already initialized", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(runner.StorePath, 0755)).To(Succeed())
		})

		It("returns an error", func() {
			err := runner.InitStore(spec)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("store already initialized at path %s", runner.StorePath)))
		})
	})

	Context("when the user is not root", func() {
		It("returns an error", func() {
			err := runner.RunningAsUser(GrootUID, GrootUID).InitStore(spec)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("store %s can only be initialized by Root user", runner.StorePath)))
		})
	})
})
