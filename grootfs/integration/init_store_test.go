package integration_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	grootfsRunner "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"golang.org/x/sys/unix"
)

var _ = Describe("Init Store", func() {
	const (
		rootUID = 0
		rootGID = 0
	)

	var (
		runner    grootfsRunner.Runner
		spec      grootfsRunner.InitSpec
		storePath string
	)

	BeforeEach(func() {
		integration.SkipIfNonRoot(GrootfsTestUid)

		storePath = filepath.Join(StorePath, strconv.Itoa(rand.Int()))
		Expect(os.MkdirAll(storePath, 0777)).To(Succeed())
		runner = Runner.WithStore(storePath).SkipInitStore()
		spec = grootfsRunner.InitSpec{}
	})

	It("returns a newly created store path", func() {
		Expect(runner.InitStore(spec)).To(Succeed())

		Expect(runner.StorePath).To(BeADirectory())

		stat, err := os.Stat(runner.StorePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
	})

	It("sets the ownership to the caller user", func() {
		Expect(runner.InitStore(spec)).To(Succeed())

		Expect(runner.StorePath).To(BeADirectory())

		var stat unix.Stat_t
		Expect(unix.Stat(runner.StorePath, &stat)).To(Succeed())
		Expect(stat.Uid).To(Equal(uint32(rootUID)))
		Expect(stat.Gid).To(Equal(uint32(rootGID)))
	})

	Context("when --store-size-bytes is passed", func() {
		var backingStoreFile string

		BeforeEach(func() {
			tmpDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			storePath = filepath.Join(tmpDir, "store")
			spec.StoreSizeBytes = 500 * 1024 * 1024
			backingStoreFile = fmt.Sprintf("%s.backing-store", storePath)

			runner = runner.WithStore(storePath)
		})

		AfterEach(func() {
			_ = unix.Unmount(storePath, 0)
			Expect(os.RemoveAll(backingStoreFile)).To(Succeed())
		})

		It("creates the backing file with the correct size", func() {
			err := runner.InitStore(spec)
			Expect(err).NotTo(HaveOccurred())

			stat, err := os.Stat(backingStoreFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Size()).To(Equal(int64(500 * 1024 * 1024)))
		})

		It("initialises a filesystem in the backing file", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("file", backingStoreFile)
			cmd.Stdout = buffer
			Expect(cmd.Run()).To(Succeed())

			Expect(buffer).To(gbytes.Say("XFS"))
		})

		Context("when the given store path is already initialized", func() {
			BeforeEach(func() {
				Expect(runner.InitStore(spec)).To(Succeed())
			})

			It("logs the event", func() {
				logs := gbytes.NewBuffer()
				Expect(runner.WithStderr(logs).InitStore(spec)).To(Succeed())

				Expect(logs).To(gbytes.Say("store-already-initialized"))
			})
		})

		Context("but the backing store file is not mounted", func() {
			BeforeEach(func() {
				Expect(runner.InitStore(spec)).To(Succeed())
				Expect(ioutil.WriteFile(filepath.Join(storePath, "test"), []byte{}, 0777)).To(Succeed())
				Expect(syscall.Unmount(storePath, 0)).To(Succeed())
				Eventually(queryStoreMountInfo(storePath)).Should(BeEmpty())
			})

			It("remounts it", func() {
				Expect(runner.InitStore(spec)).To(Succeed())
				Expect(filepath.Join(storePath, "test")).To(BeAnExistingFile())
			})
		})
	})

	Context("when id mappings are provided", func() {
		BeforeEach(func() {
			spec.UIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: GrootUID, NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}
			spec.GIDMappings = []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: GrootGID, NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			}
		})

		It("sets the ownership to the user mapped as root", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			Expect(runner.StorePath).To(BeADirectory())

			var stat unix.Stat_t
			Expect(unix.Stat(runner.StorePath, &stat)).To(Succeed())
			Expect(stat.Uid).To(Equal(uint32(GrootUID)))
			Expect(stat.Gid).To(Equal(uint32(GrootGID)))
		})
	})

	Context("when --rootless is provided", func() {
		BeforeEach(func() {
			spec.Rootless = "groot:groot"
		})

		It("sets the ownership to the provided user and group", func() {
			Expect(runner.InitStore(spec)).To(Succeed())

			Expect(runner.StorePath).To(BeADirectory())

			var stat unix.Stat_t
			Expect(unix.Stat(runner.StorePath, &stat)).To(Succeed())
			Expect(stat.Uid).To(Equal(uint32(GrootUID)))
			Expect(stat.Gid).To(Equal(uint32(GrootGID)))
		})

		It("creates a store that correctly maps the user/group ids", func() {
			sourceImagePath := integration.CreateBaseImage(0, 0, GrootUID, GrootGID)
			baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
			baseImagePath := baseImageFile.Name()

			defer func() {
				Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
				Expect(os.RemoveAll(baseImagePath)).To(Succeed())
			}()

			Expect(runner.InitStore(spec)).To(Succeed())
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           testhelpers.NewRandomID(),
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

			var stat unix.Stat_t
			Expect(unix.Stat(path.Join(containerSpec.Root.Path, "foo"), &stat)).To(Succeed())
			Expect(stat.Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(stat.Gid).To(Equal(uint32(GrootGID + 99999)))

			stat = unix.Stat_t{}
			Expect(unix.Stat(path.Join(containerSpec.Root.Path, "bar"), &stat)).To(Succeed())
			Expect(stat.Uid).To(Equal(uint32(GrootUID)))
			Expect(stat.Gid).To(Equal(uint32(GrootGID)))
		})

		Context("and id mappings are provided", func() {
			BeforeEach(func() {
				spec.UIDMappings = []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: GrootUID, NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
				}
				spec.GIDMappings = []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: GrootGID, NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
				}
			})

			It("returns an error", func() {
				err := runner.InitStore(spec)
				Expect(err).To(MatchError(ContainSubstring("cannot specify --rootless and --uid-mapping/--gid-mapping")))
			})
		})

		Context("when the rootless parameter is invalid", func() {
			BeforeEach(func() {
				spec.Rootless = "lol"
			})

			It("returns an error", func() {
				err := runner.InitStore(spec)
				Expect(err).To(MatchError(ContainSubstring("invalid --rootless parameter, format must be <user>:<group>")))
			})
		})

		Context("when the user does not exist", func() {
			BeforeEach(func() {
				spec.Rootless = "someoneelse:groot"
			})

			It("returns an error", func() {
				err := runner.InitStore(spec)
				Expect(err).To(MatchError(ContainSubstring("error reading mappings for user 'someoneelse'")))
			})
		})

		Context("when the group does not exist", func() {
			BeforeEach(func() {
				spec.Rootless = "groot:something"
			})

			It("returns an error", func() {
				err := runner.InitStore(spec)
				Expect(err).To(MatchError(ContainSubstring("error reading mappings for group 'something'")))
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
			Expect(runner.InitStore(spec)).To(Succeed())
		})

		It("does not return an error", func() {
			Expect(runner.InitStore(spec)).To(Succeed())
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

func queryStoreMountInfo(path string) func() string {
	return func() string {
		outBytes, _ := exec.Command("/bin/sh", "-c", "/bin/cat /proc/self/mountinfo | grep "+path).Output()
		return string(outBytes)
	}
}
