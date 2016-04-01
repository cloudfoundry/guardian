package gqt_test

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("InspectorGarden", func() {

	var (
		pid         string
		session     *gexec.Session
		err         error
		commandName string
		command     *exec.Cmd
		dummyCmd    *exec.Cmd
	)

	BeforeEach(func() {
		dummyCmd = exec.Command("sleep", "15")
		Expect(dummyCmd.Start()).To(Succeed())

		pid = strconv.Itoa(dummyCmd.Process.Pid)
		commandName = "/bin/sh"
	})

	AfterEach(func() {
		dummyCmd.Process.Kill()
		dummyCmd.Wait()
	})

	JustBeforeEach(func() {
		command = exec.Command(inspectorGardenBin, "-pid", pid, commandName)
	})

	Context("when passed the -pid flag", func() {
		var realgraphPath, graphPath string
		var err error

		Context("when the pid is valid", func() {
			unshareCmd := func(realPath, bindPath string) *os.Process {
				Expect(fmt.Sprintf("%s/test.txt", realPath)).ToNot(BeARegularFile())
				in := fmt.Sprintf("mount --bind %s %s && echo 'test' > %[1]s/test.txt && sleep 1000\n", realPath, bindPath)

				cmd := exec.Command("unshare", "-m", "/bin/bash")
				stdinPipe, _ := cmd.StdinPipe()
				bw := bufio.NewWriter(stdinPipe)
				bw.WriteString(in)
				bw.Flush()
				stdinPipe.Close()

				err = cmd.Start()
				Expect(err).ToNot(HaveOccurred())
				Eventually(fmt.Sprintf("%s/test.txt", realPath), "10s").Should(BeARegularFile())

				go func() {
					err = cmd.Wait()
					Expect(err).ToNot(HaveOccurred())
				}()
				return cmd.Process
			}

			BeforeEach(func() {
				realgraphPath, err = ioutil.TempDir("", "realgraph")
				Expect(err).ToNot(HaveOccurred())
				graphPath, err = ioutil.TempDir("", "graph")
				Expect(err).ToNot(HaveOccurred())

				unsharedProcess := unshareCmd(realgraphPath, graphPath)
				pid = strconv.Itoa(unsharedProcess.Pid)
			})

			It("makes the graph visible to the user", func() {
				inspectorCmdStdin, _ := command.StdinPipe()
				bw := bufio.NewWriter(inspectorCmdStdin)
				bw.WriteString("ls " + graphPath + "\n")
				bw.Flush()
				inspectorCmdStdin.Close()

				session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ToNot(HaveOccurred())
				Eventually(session.Out, "10s").Should(gbytes.Say("test.txt"))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("when the pid is invalid", func() {
			BeforeEach(func() {
				pid = "9999999"
			})

			It("exits with a 1 status code", func() {
				session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
			})
		})
	})

	Context("when the -pid flag is not passed", func() {
		JustBeforeEach(func() {
			command = exec.Command(inspectorGardenBin)
		})

		It("exits with a 1 status code", func() {
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Usage of"))
			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when passed a custom program", func() {
		Context("and the program is valid", func() {
			BeforeEach(func() {
				commandName = "/bin/bash"
			})

			It("exits with a 0 status code", func() {
				session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("and the program is invalid", func() {
			BeforeEach(func() {
				commandName = "/bin/invalid-program"
			})

			It("exits with an error", func() {
				session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
			})
		})
	})

	Context("when defining environment variable", func() {
		JustBeforeEach(func() {
			inspectorCmdStdin, _ := command.StdinPipe()
			bw := bufio.NewWriter(inspectorCmdStdin)
			bw.WriteString("printenv \n")
			bw.Flush()
			inspectorCmdStdin.Close()
		})

		It("keeps it on the new namespace", func() {
			command.Env = []string{"ENV_USER=alice"}
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(gbytes.Say("ENV_USER=alice"))
		})

		It("exports PS1 env variable", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(gbytes.Say("PS1=inspector-garden#"))
		})
	})

})
