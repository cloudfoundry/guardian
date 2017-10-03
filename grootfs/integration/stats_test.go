package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Stats", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		containerSpec   specs.Spec
		imageID         string
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		imageID = testhelpers.NewRandomID()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
	})

	Context("when the store doesn't exist", func() {
		It("logs the image path", func() {
			logBuffer := gbytes.NewBuffer()
			_, err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).
				Stats("/path/to/random-id")
			Expect(err).To(HaveOccurred())
			Expect(logBuffer).To(gbytes.Say(`"id":"/path/to/random-id"`))
		})
	})

	Context("when image exists", func() {
		var (
			expectedStats groot.VolumeStats
			diskLimit     int64
		)

		BeforeEach(func() {
			diskLimit = 1024 * 1024 * 50
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(sourceImagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		JustBeforeEach(func() {
			var err error
			containerSpec, err = Runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           imageID,
				DiskLimit:    diskLimit,
				Mount:        isBtrfs(), // btrfs needs the mount option
			})
			Expect(err).ToNot(HaveOccurred())

			writeFileCmdLine := fmt.Sprintf("dd if=/dev/zero of=%s bs=1048576 count=4", filepath.Join(containerSpec.Root.Path, "hello"))

			var cmd *exec.Cmd
			if containerSpec.Mounts != nil {
				cmd = unshareWithMount(writeFileCmdLine, containerSpec.Mounts[0])
			} else {
				cmd = exec.Command("sh", "-c", writeFileCmdLine)
			}

			sess := runAsUser(cmd, GrootfsTestUid, GrootfsTestGid)
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))

			if Driver == "overlay-xfs" {
				expectedStats = groot.VolumeStats{
					DiskUsage: groot.DiskUsage{
						TotalBytesUsed:     9445376,
						ExclusiveBytesUsed: 4202496,
					},
				}
			} else {
				expectedStats = groot.VolumeStats{
					DiskUsage: groot.DiskUsage{
						TotalBytesUsed:     9441280,
						ExclusiveBytesUsed: 4198400,
					},
				}
			}
		})

		Context("when the last parameter is the image ID", func() {
			It("returns the stats for given image id", func() {
				stats, err := Runner.Stats(imageID)
				Expect(err).NotTo(HaveOccurred())

				Expect(stats.DiskUsage.TotalBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.TotalBytesUsed, 100),
				)
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.ExclusiveBytesUsed, 100),
				)
			})
		})

		Context("when the last parameter is the image path", func() {
			It("returns the stats for given image path", func() {
				stats, err := Runner.Stats(filepath.Dir(containerSpec.Root.Path))
				Expect(err).NotTo(HaveOccurred())

				Expect(stats.DiskUsage.TotalBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.TotalBytesUsed, 100),
				)
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.ExclusiveBytesUsed, 100),
				)
			})
		})

		Context("when aux binary doesn't have the suid bit", func() {
			var (
				tardisBin, draxBin string
				runner             runner.Runner
			)

			BeforeEach(func() {
				draxBin, err := gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax")
				Expect(err).NotTo(HaveOccurred())
				draxBin = integration.MakeBinaryAccessibleToEveryone(draxBin)
				tardisBin, err := gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis")
				Expect(err).NotTo(HaveOccurred())
				tardisBin = integration.MakeBinaryAccessibleToEveryone(tardisBin)

				runner = Runner.WithDraxBin(draxBin).WithTardisBin(tardisBin)
			})

			AfterEach(func() {
				Expect(os.RemoveAll(tardisBin)).To(Succeed())
				Expect(os.RemoveAll(draxBin)).To(Succeed())
			})

			Context("when running as root user", func() {
				BeforeEach(func() {
					integration.SkipIfNonRoot(GrootfsTestUid)
				})

				It("succeeds", func() {
					_, err := runner.Stats(filepath.Dir(containerSpec.Root.Path))
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when running as non-root user", func() {
				BeforeEach(func() {
					integration.SkipIfRoot(GrootfsTestUid)
				})

				It("returns an error", func() {
					_, err := runner.Stats(filepath.Dir(containerSpec.Root.Path))
					Expect(err.Error()).To(ContainSubstring("missing the setuid bit"))
				})
			})
		})

		Context("when the image has no quotas", func() {
			BeforeEach(func() {
				integration.SkipIfNotXFS(Driver)
				diskLimit = 0
			})

			It("returns 0 as exclusive usage", func() {
				stats, err := Runner.Stats(imageID)
				Expect(err).NotTo(HaveOccurred())

				Expect(stats.DiskUsage.TotalBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.TotalBytesUsed-expectedStats.DiskUsage.ExclusiveBytesUsed, 100),
				)
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(
					BeNumerically("~", 0, 100),
				)
			})
		})
	})

	Context("when the image id doesn't exist", func() {
		Context("when the last parameter is a image id", func() {
			It("returns an error", func() {
				_, err := Runner.Stats("invalid-id")
				Expect(err).To(MatchError(ContainSubstring("Image `invalid-id` not found. Skipping delete.")))
			})
		})

		Context("when the last parameter is a path", func() {
			It("returns an error", func() {
				invalidImagePath := filepath.Join(StorePath, store.ImageDirName, "not-here")
				_, err := Runner.Stats(invalidImagePath)
				Expect(err).To(MatchError(ContainSubstring("Image `not-here` not found. Skipping delete.")))
			})

			Context("when the path provided doesn't belong to the `--store` provided", func() {
				It("returns an error", func() {
					_, err := Runner.Stats("/Iamnot/in/the/storage/images/1234/rootfs")
					Expect(err).To(MatchError(ContainSubstring("path `/Iamnot/in/the/storage/images/1234/rootfs` is outside store path")))
				})
			})
		})
	})

	Context("when the image id is not provided", func() {
		It("returns an error", func() {
			_, err := Runner.Stats("")
			Expect(err).To(MatchError(ContainSubstring("invalid arguments")))
		})
	})
})

func unshareWithMount(cmdLine string, mount specs.Mount) *exec.Cmd {
	mountOptions := strings.Join(mount.Options, ",")
	mountCmdLine := fmt.Sprintf("mount -t %s %s -o%s %s", mount.Type, mount.Source, mountOptions, mount.Destination)

	return exec.Command("unshare", "--user", "--map-root-user", "--mount", "sh", "-c",
		fmt.Sprintf("%s; %s", mountCmdLine, cmdLine))
}

func runAsUser(cmd *exec.Cmd, uid, gid int) *gexec.Session {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return sess

}
