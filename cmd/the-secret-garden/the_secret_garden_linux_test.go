package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("The Secret Garden", func() {
	var (
		stubProcess    string
		pidFile        string
		fakeDataDir    string
		realGraphDir   string
		secretGraphDir string
		session        *gexec.Session
	)

	runSecretGarden := func(dataDir, realGraphDir, secretGraphDir, bin string, args ...string) *gexec.Session {
		cmd := exec.Command(theSecretGardenBin, append([]string{dataDir, realGraphDir, secretGraphDir, bin}, args...)...)
		cmd.Env = append(os.Environ(), "BAR=foo")
		sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		return sess
	}

	BeforeEach(func() {
		var err error
		fakeDataDir, err = ioutil.TempDir("", fmt.Sprintf("data-%d", GinkgoParallelNode()))
		Expect(err).NotTo(HaveOccurred())

		pidFile = filepath.Join(fakeDataDir, "pidfile")

		realGraphDir = filepath.Join(fakeDataDir, "realgraph")
		Expect(os.MkdirAll(realGraphDir, 0777)).To(Succeed())

		secretGraphDir = filepath.Join(fakeDataDir, "graph")
		Expect(os.MkdirAll(secretGraphDir, 0777)).To(Succeed())
	})

	It("writes the namespaced process id to a pidfile", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "sh", "-c", "echo $$")

		Eventually(session).Should(gexec.Exit(0))
		Expect(pidFile).To(BeARegularFile())

		actualPid := strings.TrimSpace(string(session.Out.Contents()))

		pidBuf, err := ioutil.ReadFile(pidFile)
		Expect(err).NotTo(HaveOccurred())

		Expect(string(pidBuf)).To(Equal(actualPid))
	})

	It("blows up when bad pidfile is passed", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "some/bad/pid", "pwd")
		Eventually(session).Should(gexec.Exit(1))
	})

	It("makes sure the data dir is mounted exactly once", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "mount")
		Eventually(session).Should(gexec.Exit(0))

		exp := fmt.Sprintf("on %s type", fakeDataDir)
		Expect(session.Out).To(gbytes.Say(exp))

		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "mount")
		Eventually(session).Should(gexec.Exit(0))

		Expect(regexp.MustCompile(exp).FindAll(session.Out.Contents(), -1)).To(HaveLen(1))
	})

	It("mounts the realgraph/graph on the graph path exactly once", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "mount")
		Eventually(session).Should(gexec.Exit(0))

		exp := fmt.Sprintf("on %s type", secretGraphDir)
		output := session.Out.Contents()
		Expect(output).To(ContainSubstring(exp))

		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "mount")
		Eventually(session).Should(gexec.Exit(0))

		Expect(regexp.MustCompile(exp).FindAll(session.Out.Contents(), -1)).To(HaveLen(1))
	})

	It("exits non-zero when it fails to mount --make-shared", func() {
		session = runSecretGarden("nnonexistent-dir", realGraphDir, secretGraphDir, pidFile, "pwd")
		Eventually(session).Should(gexec.Exit(1))
	})

	It("changes the realGraphDir's permission accordingly", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "sh", "-c", fmt.Sprintf("stat -c %%A %s", realGraphDir))
		Eventually(session).Should(gexec.Exit(0))

		// Checking if the x's are removed from the group and other
		// the os removes the write permissions of group and other
		exp := fmt.Sprintf("drwxr.-r.-")
		Expect(session.Out).To(gbytes.Say(exp))
	})

	It("exits non-zero when it fails to mount the graph", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, "no-such-dir", secretGraphDir, pidFile, "mount")
		Eventually(session).Should(gexec.Exit(1))
	})

	It("exits non-zero when the command it execs fails", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "no-such-cmd")
		Eventually(session).Should(gexec.Exit(1))
	})

	It("passes the correct environment to the final process", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, "sh", "-c", "echo $BAR")
		Eventually(session.Out).Should(gbytes.Say("foo"))
		Eventually(session).Should(gexec.Exit(0))
	})

	Context("when a mount is created outside the namespace after the secret garden is started", func() {
		const accessSharedMount = `#!/bin/bash
			set -x
			for i in $(seq 1 10); do
				echo trying $i
				sleep 1
				stat ${1}/myfile
				if [[ $? -eq 0 ]]; then
					exit
				fi
			done
		`
		var sharedDir string

		BeforeEach(func() {
			stubProcess = filepath.Join(fakeDataDir, "access-mount.sh")
			sharedDir = filepath.Join(fakeDataDir, "shared")
			Expect(ioutil.WriteFile(stubProcess, []byte(accessSharedMount), 0777)).To(Succeed())
		})

		It("is visible inside the unshared namespace", func() {
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, stubProcess, sharedDir)
			Eventually(func() string {
				out, err := exec.Command("mount").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				return string(out)
			}).Should(ContainSubstring(fmt.Sprintf("on %s type", fakeDataDir)))

			Expect(exec.Command("mkdir", sharedDir).Run()).To(Succeed())
			Expect(exec.Command("mount", "-t", "tmpfs", "tmpfs", sharedDir).Run()).To(Succeed())

			Expect(exec.Command("touch", filepath.Join(sharedDir, "myfile")).Run()).To(Succeed())
			Expect(exec.Command("stat", filepath.Join(sharedDir, "myfile")).Run()).To(Succeed())

			Eventually(session, "11s").Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("shared/myfile"))
		})
	})

	Context("when a mount is created outside the namespace before the secret garden is started", func() {
		const accessSharedMount = `#!/bin/bash
			set -x
			for i in $(seq 1 10); do
				echo trying $i
				sleep 1
				stat ${1}/myfile
				if [[ $? -eq 0 ]]; then
					exit
				fi
			done
		`
		var sharedDir string

		BeforeEach(func() {
			stubProcess = filepath.Join(fakeDataDir, "access-mount.sh")
			sharedDir = filepath.Join(fakeDataDir, "shared")
			Expect(ioutil.WriteFile(stubProcess, []byte(accessSharedMount), 0777)).To(Succeed())
		})

		It("is visible inside the unshared namespace", func() {
			Expect(exec.Command("mkdir", sharedDir).Run()).To(Succeed())
			Expect(exec.Command("mount", "-t", "tmpfs", "tmpfs", sharedDir).Run()).To(Succeed())

			Expect(exec.Command("touch", filepath.Join(sharedDir, "myfile")).Run()).To(Succeed())
			Expect(exec.Command("stat", filepath.Join(sharedDir, "myfile")).Run()).To(Succeed())

			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, stubProcess, sharedDir)
			Eventually(func() string {
				out, err := exec.Command("mount").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				return string(out)
			}).Should(ContainSubstring(fmt.Sprintf("on %s type", fakeDataDir)))

			Eventually(session, "11s").Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("shared/myfile"))
		})
	})

	Context("when the process creates a file in the secretGraphDir", func() {
		const makeSecretMount = `#!/bin/sh
			set -e -x
			touch ${1}/mysecret
			echo -n password > ${1}/mysecret
		`

		BeforeEach(func() {
			stubProcess = filepath.Join(fakeDataDir, "create-mount.sh")
			Expect(ioutil.WriteFile(stubProcess, []byte(makeSecretMount), 0777)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.Remove(stubProcess)).To(Succeed())
		})

		It("prevents the file to be seen from outside the namespace", func() {
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, stubProcess, secretGraphDir)

			Consistently(func() []os.FileInfo {
				fileInfo, _ := ioutil.ReadDir(secretGraphDir)
				return fileInfo
			}, time.Second*3).Should(BeEmpty())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when the process creates a mount", func() {
		const makeSecretMount = `#!/bin/sh
			set -e -x
			mkdir -p ${1}
			mount -t tmpfs tmpfs ${1}
			touch ${1}/mysecret
			echo -n password > ${1}/mysecret
		`

		var secretDir string

		BeforeEach(func() {
			stubProcess = filepath.Join(fakeDataDir, "create-mount.sh")
			secretDir = filepath.Join(fakeDataDir, "secret")
			Expect(ioutil.WriteFile(stubProcess, []byte(makeSecretMount), 0777)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.Remove(stubProcess)).To(Succeed())
		})

		It("prevents the mount to be seen from outside the namespace", func() {
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, stubProcess, secretDir)

			Consistently(func() []os.FileInfo {
				fileInfo, _ := ioutil.ReadDir(secretDir)
				return fileInfo
			}).Should(BeEmpty())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when the secret garden is killed", func() {
		var sharedDir string

		const processes = `#!/bin/bash
			echo "PID: $$"
			read
		`

		BeforeEach(func() {
			stubProcess = filepath.Join(fakeDataDir, "spawn-processes.sh")
			sharedDir = filepath.Join(fakeDataDir, "shared")
			Expect(ioutil.WriteFile(stubProcess, []byte(processes), 0777)).To(Succeed())
		})

		It("should kill the underlying process", func() {
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, pidFile, stubProcess, sharedDir)
			Eventually(session.Out).Should(gbytes.Say("PID: "))

			contents, err := ioutil.ReadAll(session.Out)
			Expect(err).NotTo(HaveOccurred())

			pid, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 32)
			Expect(err).NotTo(HaveOccurred())

			session.Kill()

			Eventually(fmt.Sprintf("/proc/%d", pid)).ShouldNot(BeADirectory())
		})
	})
})
