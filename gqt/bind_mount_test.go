package gqt_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Bind mount", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container

		// container create parms
		privilegedContainer bool
		srcPath             string                 // bm: source
		dstPath             string                 // bm: destination
		bindMountMode       garden.BindMountMode   // bm: RO or RW
		bindMountOrigin     garden.BindMountOrigin // bm: Container or Host

		// pre-existing file for permissions testing
		testFileName string
	)

	allBridges := func() []byte {
		stdout := gbytes.NewBuffer()
		cmd, err := gexec.Start(exec.Command("ip", "a"), stdout, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		cmd.Wait(time.Second * 5)

		return stdout.Contents()
	}

	BeforeEach(func() {
		privilegedContainer = false
		container = nil
		srcPath = ""
		dstPath = ""
		bindMountMode = garden.BindMountModeRO
		bindMountOrigin = garden.BindMountOriginHost
		testFileName = ""
	})

	JustBeforeEach(func() {
		client = startGarden()

		var err error
		container, err = client.Create(
			garden.ContainerSpec{
				Privileged: privilegedContainer,
				BindMounts: []garden.BindMount{garden.BindMount{
					SrcPath: srcPath,
					DstPath: dstPath,
					Mode:    bindMountMode,
					Origin:  bindMountOrigin,
				}},
				Network: fmt.Sprintf("10.0.%d.0/24", GinkgoParallelNode()),
			})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if container != nil {
			err := client.Destroy(container.Handle())
			Expect(err).ToNot(HaveOccurred())
		}

		// sanity check that bridges were cleaned up
		bridgePrefix := fmt.Sprintf("w%db-", GinkgoParallelNode())
		Expect(allBridges()).ToNot(ContainSubstring(bridgePrefix))

		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("with a host origin bind-mount", func() {
		BeforeEach(func() {
			srcPath, testFileName = createTestHostDirAndTestFile()
			bindMountOrigin = garden.BindMountOriginHost
		})

		AfterEach(func() {
			err := os.RemoveAll(srcPath)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("which is read-only", func() {
			BeforeEach(func() {
				bindMountMode = garden.BindMountModeRO
				dstPath = "/home/alice/readonly"
			})

			Context("and with privileged=true", func() {
				BeforeEach(func() {
					privilegedContainer = true
				})

				It("is successfully created with correct privileges for non-root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, false)
				})

				It("is successfully created with correct privileges for root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, true)
				})
			})

			Context("and with privileged=false", func() {
				BeforeEach(func() {
					privilegedContainer = false
				})

				It("is successfully created with correct privileges for non-root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, false)
				})

				It("is successfully created with correct privileges for root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, true)
				})
			})
		})

		Context("which is read-write", func() {
			BeforeEach(func() {
				bindMountMode = garden.BindMountModeRW
				dstPath = "/home/alice/readwrite"
			})

			Context("and with privileged=true", func() {
				BeforeEach(func() {
					privilegedContainer = true
				})

				It("is successfully created with correct privileges for non-root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, false)
				})

				It("is successfully created with correct privileges for root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, true)
				})
			})

			Context("and with privileged=false", func() {
				BeforeEach(func() {
					privilegedContainer = false
				})

				It("is successfully created with correct privileges for non-root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, false)
				})

				PIt("is successfully created with correct privileges for root in container", func() {
					checkFileAccess(container, bindMountMode, bindMountOrigin, dstPath, testFileName, privilegedContainer, true)
				})
			})
		})
	})
})

func createTestHostDirAndTestFile() (string, string) {
	tstHostDir, err := ioutil.TempDir("", "bind-mount-test-dir")
	Expect(err).ToNot(HaveOccurred())
	err = os.Chown(tstHostDir, 0, 0)
	Expect(err).ToNot(HaveOccurred())
	err = os.Chmod(tstHostDir, 0755)
	Expect(err).ToNot(HaveOccurred())

	fileName := fmt.Sprintf("bind-mount-%d-test-file", GinkgoParallelNode())
	file, err := os.OpenFile(filepath.Join(tstHostDir, fileName), os.O_CREATE|os.O_RDWR, 0777)
	Expect(err).ToNot(HaveOccurred())
	Expect(file.Close()).ToNot(HaveOccurred())

	return tstHostDir, fileName
}

func checkFileAccess(container garden.Container, bindMountMode garden.BindMountMode, bindMountOrigin garden.BindMountOrigin, dstPath string, fileName string, privCtr, privReq bool) {
	readOnly := (garden.BindMountModeRO == bindMountMode)
	ctrOrigin := (garden.BindMountOriginContainer == bindMountOrigin)
	realRoot := (privReq && privCtr)

	// can we read a file?
	filePath := filepath.Join(dstPath, fileName)

	var user string
	if privReq {
		user = "root"
	} else {
		user = "alice"
	}

	process, err := container.Run(garden.ProcessSpec{
		Path: "cat",
		Args: []string{filePath},
		User: user,
	}, garden.ProcessIO{})
	Expect(err).ToNot(HaveOccurred())

	Expect(process.Wait()).To(Equal(0))

	// try to write a new file
	filePath = filepath.Join(dstPath, "checkFileAccess-file")

	process, err = container.Run(garden.ProcessSpec{
		Path: "touch",
		Args: []string{filePath},
		User: user,
	}, garden.ProcessIO{
		Stderr: GinkgoWriter,
		Stdout: GinkgoWriter,
	})
	Expect(err).ToNot(HaveOccurred())

	if readOnly || (!realRoot && !ctrOrigin) {
		Expect(process.Wait()).ToNot(Equal(0))
	} else {
		Expect(process.Wait()).To(Equal(0))
	}

	// try to delete an existing file
	filePath = filepath.Join(dstPath, fileName)

	process, err = container.Run(garden.ProcessSpec{
		Path: "rm",
		Args: []string{filePath},
		User: user,
	}, garden.ProcessIO{})
	Expect(err).ToNot(HaveOccurred())
	if readOnly || (!realRoot && !ctrOrigin) {
		Expect(process.Wait()).ToNot(Equal(0))
	} else {
		Expect(process.Wait()).To(Equal(0))
	}
}
