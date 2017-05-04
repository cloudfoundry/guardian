package gqt_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/pkg/locksmith"
	"code.cloudfoundry.org/idmapper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("gdn setup", func() {
	var (
		cgroupsMountpoint string
		iptablesPrefix    string
		setupArgs         []string
		tag               string
		runcRoot          string
		setupProcess      *gexec.Session
	)

	BeforeEach(func() {
		// we can't use GinkgoParallelNode() directly here as this causes interference with the other tests in the GQT suite
		// i.e. beacuse for these specific tests, we want to teardown the iptables/cgroups after each test
		// and also because the --tag has a limitation of 1 char in length
		tag = nodeToString(GinkgoParallelNode())
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", tag))
		iptablesPrefix = fmt.Sprintf("w-%s", tag)
		runcRoot = fmt.Sprintf("/tmp/runc-root-%s", tag)
		setupArgs = []string{"setup", "--tag", tag, "--runc-root", runcRoot}
	})

	JustBeforeEach(func() {
		var err error

		setupProcess, err = gexec.Start(exec.Command(gardenBin, setupArgs...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess, 10*time.Second).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(cleanupSystemResources(cgroupsMountpoint, iptablesPrefix)).To(Succeed())
		Expect(os.RemoveAll(runcRoot))
	})

	It("sets up cgroups", func() {
		mountpointCmd := exec.Command("mountpoint", "-q", cgroupsMountpoint+"/")
		Expect(mountpointCmd.Run()).To(Succeed())
	})

	It("doesn't log spurious messages", func() {
		Consistently(setupProcess).ShouldNot(gbytes.Say("guardian-setup.iptables-runner.command.failed"))
	})

	Context("when we start the server", func() {
		var (
			server     *runner.RunningGarden
			serverArgs []string
		)

		BeforeEach(func() {
			serverArgs = []string{"--skip-setup", "--tag", fmt.Sprintf("%s", tag)}
		})

		AfterEach(func() {
			Expect(server.DestroyAndStop()).To(Succeed())
		})

		Context("when the server is running as root", func() {
			JustBeforeEach(func() {
				root := &syscall.Credential{Uid: 0, Gid: 0}
				server = startGardenAsUser(root, serverArgs...)
				Expect(server).NotTo(BeNil())
			})

			It("should be able to create a container", func() {
				_, err := server.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when a dummy network plugin is suppplied", func() {
				BeforeEach(func() {
					serverArgs = append(serverArgs, []string{"--network-plugin", "/bin/true"}...)
				})

				It("should be able to create a container", func() {
					_, err := server.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})

	Context("when run without --experimental-allow-rootless-for-uid <uid>", func() {
		Context("and the runc root already exists", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(runcRoot, 0700)).To(Succeed())
				Expect(os.Chown(runcRoot, 1000, 1000)).To(Succeed())
			})

			It("preserves the ownership on the runc root dir", func() {
				info, err := os.Stat(runcRoot)
				Expect(err).NotTo(HaveOccurred())

				Expect(info.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1000)))
				Expect(info.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(1000)))
			})
		})
	})

	Context("when run with --experimental-allow-rootless-for-uid <uid>", func() {
		var uid int

		BeforeEach(func() {
			uid = 1001 + GinkgoParallelNode()
			setupArgs = append(setupArgs, "--experimental-allow-rootless-for-uid", fmt.Sprintf("%d", uid))
		})

		Describe("the runc root dir", func() {
			AfterEach(func() {
				Expect(os.RemoveAll(runcRoot)).To(Succeed())
			})

			Context("when the dir exists", func() {
				BeforeEach(func() {
					Expect(os.MkdirAll(runcRoot, 0700)).To(Succeed())
				})

				Context("but it is not owned by the provided uid", func() {
					BeforeEach(func() {
						Expect(os.Chown(runcRoot, 0, 0)).To(Succeed())
					})

					It("sets the ownership of the dir to the provided uid", func() {
						info, err := os.Stat(runcRoot)
						Expect(err).NotTo(HaveOccurred())

						Expect(info.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
						Expect(info.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
						Expect(info.Mode().Perm()).To(Equal(os.FileMode(0700)))
					})
				})
			})

			Context("when the dir doesn't exist", func() {
				It("creates it with the correct permissions and ownership on the dir", func() {
					info, err := os.Stat(runcRoot)
					Expect(err).NotTo(HaveOccurred())

					Expect(info.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
					Expect(info.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
					Expect(info.Mode().Perm()).To(Equal(os.FileMode(0700)))
				})
			})

			Describe("the gdn run dir", func() {
				var gdnRunDir string

				BeforeEach(func() {
					gdnRunDir = fmt.Sprintf("/var/run/user/%d/gdn", uid)
				})

				AfterEach(func() {
					Expect(os.RemoveAll(gdnRunDir)).To(Succeed())
				})

				Context("when the dir exists", func() {
					BeforeEach(func() {
						Expect(os.MkdirAll(gdnRunDir, 0700)).To(Succeed())
					})

					Context("but it is not owned by the provided uid", func() {
						BeforeEach(func() {
							Expect(os.Chown(gdnRunDir, 0, 0)).To(Succeed())
						})

						It("sets the ownership of the dir to the provided uid", func() {
							info, err := os.Stat(gdnRunDir)
							Expect(err).NotTo(HaveOccurred())

							Expect(info.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
							Expect(info.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
							Expect(info.Mode().Perm()).To(Equal(os.FileMode(0700)))
						})
					})
				})

				Context("when the dir doesn't exist", func() {
					It("creates it with the correct permissions and ownership on the dir", func() {
						info, err := os.Stat(gdnRunDir)
						Expect(err).NotTo(HaveOccurred())

						Expect(info.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
						Expect(info.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
						Expect(info.Mode().Perm()).To(Equal(os.FileMode(0700)))
					})
				})
			})

			itChecksTheSubidFiles := func(id string) {
				Describe(fmt.Sprintf("the /etc/sub%sid file", id), func() {
					Context("when the provided uid does not exist in the file", func() {
						BeforeEach(func() {
							mutex := locksmith.NewFileSystem()
							unlocker, err := mutex.Lock("/tmp/subid-lock")
							Expect(err).NotTo(HaveOccurred())
							defer unlocker.Unlock()

							subidFile, err := os.OpenFile(fmt.Sprintf("/etc/sub%sid", id), os.O_WRONLY|os.O_TRUNC, 0644)
							Expect(err).NotTo(HaveOccurred())
							defer subidFile.Close()
							subidBackupFile, err := os.Open(fmt.Sprintf("/etc/sub%sid-", id))
							Expect(err).NotTo(HaveOccurred())
							defer subidBackupFile.Close()
							_, err = io.Copy(subidFile, subidBackupFile)
							Expect(err).NotTo(HaveOccurred())
						})

						It("prints a warning", func() {
							Eventually(setupProcess).Should(gbytes.Say("WARNING: uid %d does not have permission to map the entire %sID range", uid, strings.ToUpper(id)))
						})
					})

					Context("when the provided uid can map the entire range", func() {
						BeforeEach(func() {
							mutex := locksmith.NewFileSystem()
							unlocker, err := mutex.Lock("/tmp/subid-lock")
							Expect(err).NotTo(HaveOccurred())
							defer unlocker.Unlock()

							subidFile, err := os.OpenFile(fmt.Sprintf("/etc/sub%sid", id), os.O_WRONLY|os.O_APPEND, 0644)
							Expect(err).NotTo(HaveOccurred())
							defer subidFile.Close()
							var maxID int
							if id == "u" {
								maxID = idmapper.MustGetMaxValidUID()
							} else if id == "g" {
								maxID = idmapper.MustGetMaxValidGID()
							}
							Expect(maxID).NotTo(BeZero())
							systemIDCount := maxID + 1
							fmt.Fprintf(subidFile, "%d:0:%d", uid, systemIDCount)
						})

						It("prints no warning", func() {
							Expect(string(setupProcess.Buffer().Contents())).NotTo(ContainSubstring("WARNING: uid %d does not have permission to map the entire %ID range", uid, strings.ToUpper(id)))
						})
					})
				})
			}

			itChecksTheSubidFiles("u")
			itChecksTheSubidFiles("g")
		})
	})
})
