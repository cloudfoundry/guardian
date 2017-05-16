package integration_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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
		integration.SkipIfNonRoot(GrootfsTestUid)

		spec = manager.InitSpec{}
		storePath = filepath.Join(StorePath, fmt.Sprintf("init-store-%d", rand.Int()))
		runner = Runner.WithStore(storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath))
	})

	Context("when we need to create the store filesystem", func() {
		var backingStoreFile string

		BeforeEach(func() {
			integration.SkipIfNotXFS(Driver)
			Expect(os.MkdirAll(storePath, 0755)).To(Succeed())

			storePath = filepath.Join(storePath, "store")
			runner = Runner.WithStore(storePath)
			spec.StoreSizeBytes = 500 * 1024 * 1024
			backingStoreFile = fmt.Sprintf("%s.backing-store", storePath)
		})

		AfterEach(func() {
			syscall.Unmount(storePath, 0)
			Expect(os.RemoveAll(backingStoreFile)).To(Succeed())
		})

		It("creates the backing file with the correct size", func() {
			err := runner.InitStore(spec)
			Expect(err).NotTo(HaveOccurred())

			stat, err := os.Stat(backingStoreFile)
			Expect(err).To(BeNil())
			Expect(stat.Size()).To(Equal(int64(500 * 1024 * 1024)))
		})

		It("initialises an XFS filesystem in the backing file", func() {
			Expect(runner.WithLogLevel(lager.DEBUG).InitStore(spec)).To(Succeed())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("file", backingStoreFile)
			cmd.Stdout = buffer
			Expect(cmd.Run()).To(Succeed())
			Expect(buffer).To(gbytes.Say("XFS"))
		})

		Context("when the given backing store size is too small", func() {
			BeforeEach(func() {
				spec.StoreSizeBytes = 199 * 1024 * 1024
			})

			It("returns an error", func() {
				err := runner.InitStore(spec)
				Expect(err).To(MatchError(ContainSubstring("store size must be at least 200Mb")))
			})
		})

		Context("when backing file already exists", func() {
			BeforeEach(func() {
				backingStoreFile = fmt.Sprintf("%s.backing-store", storePath)
				ioutil.WriteFile(backingStoreFile, []byte{}, 0600)
			})

			It("fails with nice error message", func() {
				err := runner.InitStore(spec)
				Expect(err).To(MatchError(ContainSubstring("backing store file already exists at path")))
			})
		})
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
			Mount:     true,
		})

		Expect(err).ToNot(HaveOccurred())
	})

	Context("when the store is used to create a image with different mappings", func() {
		It("fails", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			_, err := runner.Create(groot.CreateSpec{
				ID:        "random-id",
				BaseImage: "docker:///cfgarden/empty",
				Mount:     true,
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
				Mount:     true,
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
					Mount:     true,
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
