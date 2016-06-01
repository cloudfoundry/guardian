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

		realGraphDir = filepath.Join(fakeDataDir, "realgraph")
		Expect(os.MkdirAll(realGraphDir, 0777)).To(Succeed())

		secretGraphDir = filepath.Join(fakeDataDir, "graph")
		Expect(os.MkdirAll(secretGraphDir, 0777)).To(Succeed())
	})

	AfterEach(func() {
		exec.Command("umount", fakeDataDir).Run()
		os.RemoveAll(fakeDataDir)
	})

	It("makes sure the data dir is mounted exactly once", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "mount")
		Eventually(session).Should(gexec.Exit(0))

		exp := fmt.Sprintf("%s on %s", fakeDataDir, fakeDataDir)
		Expect(session.Out).To(gbytes.Say(exp))

		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "mount")
		Eventually(session).Should(gexec.Exit(0))

		Expect(regexp.MustCompile(exp).FindAll(session.Out.Contents(), -1)).To(HaveLen(1))
	})

	It("mounts the realgraph/graph on the graph path exactly once", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "mount")
		Eventually(session).Should(gexec.Exit(0))

		exp := fmt.Sprintf("%s on %s", filepath.Join(realGraphDir, "graph"), secretGraphDir)
		Expect(session.Out).To(gbytes.Say(exp))

		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "mount")
		Eventually(session).Should(gexec.Exit(0))

		Expect(regexp.MustCompile(exp).FindAll(session.Out.Contents(), -1)).To(HaveLen(1))
	})

	It("exits non-zero when it fails to mount --make-shared", func() {
		session = runSecretGarden("nnonexistent-dir", realGraphDir, secretGraphDir, "pwd")
		Eventually(session).ShouldNot(gexec.Exit(0))
	})

	It("changes the realGraphDir's permission accordingly", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "sh", "-c", fmt.Sprintf("stat -c %%A %s", realGraphDir))
		Eventually(session).Should(gexec.Exit(0))

		// Checking if the x's are removed from the group and other
		// the os removes the write permissions of group and other
		exp := fmt.Sprintf("drwxr.-r.-")
		Expect(session.Out).To(gbytes.Say(exp))
	})

	It("exits non-zero when the realGraphDir is not a valid folder", func() {
		session = runSecretGarden(fakeDataDir, "spiderman-kaput-dir", secretGraphDir, "ls")
		Eventually(session).ShouldNot(gexec.Exit(0))
	})

	It("exits non-zero when it fails to mount the graph", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, "no-such-dir", secretGraphDir, "mount")
		Eventually(session).ShouldNot(gexec.Exit(0))
	})

	It("exits non-zero when the command it execs fails", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "no-such-cmd")
		Eventually(session).ShouldNot(gexec.Exit(0))
	})

	It("passes the correct environment to the final process", func() {
		session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, "sh", "-c", "echo $BAR")
		Eventually(session.Out).Should(gbytes.Say("foo"))
		Eventually(session).ShouldNot(gexec.Exit(0))
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
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, stubProcess, secretGraphDir)

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
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, stubProcess, secretDir)

			Consistently(func() []os.FileInfo {
				fileInfo, _ := ioutil.ReadDir(secretDir)
				return fileInfo
			}).Should(BeEmpty())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when the secret garden is terminated", func() {
		var sharedDir string

		const processes = `#!/bin/bash
			trap "echo terminating; exit 0" SIGTERM

			echo 'sleeping'
			for i in $(seq 1 1000); do
			  sleep 1
			done

		`

		BeforeEach(func() {
			stubProcess = filepath.Join(fakeDataDir, "spawn-processes.sh")
			sharedDir = filepath.Join(fakeDataDir, "shared")
			Expect(ioutil.WriteFile(stubProcess, []byte(processes), 0777)).To(Succeed())
		})

		It("should kill the underlying process", func() {
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, stubProcess, sharedDir)
			Eventually(session.Out).Should(gbytes.Say("sleeping"))

			session.Terminate()

			Eventually(session.Out, "3s").Should(gbytes.Say("terminating"))
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
			session = runSecretGarden(fakeDataDir, realGraphDir, secretGraphDir, stubProcess, sharedDir)
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
