package gqt_test

import (
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("runC Logging", func() {
	var (
		client   *runner.RunningGarden
		logLevel string
	)

	JustBeforeEach(func() {
		binPath, err := gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/fake_runc")
		Expect(err).NotTo(HaveOccurred())

		containerSpec := garden.ContainerSpec{}

		client = startGarden("--runc-bin", binPath, "--log-level", logLevel)
		client.Create(containerSpec)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("when the log-level is set to debug", func() {
		BeforeEach(func() {
			logLevel = "debug"
		})

		It("logs runc logs to stdout", func() {
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-info"))
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-warn"))
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-error"))
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-print"))
		})
	})

	Context("when the log-level is set to info", func() {
		BeforeEach(func() {
			logLevel = "info"
		})

		It("does not log runc logs to stdout", func() {
			Consistently(client, "1s").ShouldNot(gbytes.Say("guardian-runc-logging-test-info"))
			Consistently(client, "1s").ShouldNot(gbytes.Say("guardian-runc-logging-test-warn"))
			Consistently(client, "1s").ShouldNot(gbytes.Say("guardian-runc-logging-test-error"))
			Consistently(client, "1s").ShouldNot(gbytes.Say("guardian-runc-logging-test-print"))
		})
	})
})

var _ = Describe("garden server Logging", func() {
	var (
		args   []string
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		args = []string{}
	})

	JustBeforeEach(func() {
		client = startGarden(args...)
	})

	Context("when the log-level is passed as debug", func() {
		BeforeEach(func() {
			args = []string{"--log-level", "debug"}
		})

		AfterEach(func() {
			Expect(client.Stop()).To(Succeed())
		})

		It("logs at debug level", func() {
			Eventually(client, "1s").Should(gbytes.Say(`"log_level":0`))
		})
	})

	Context("when log-level is not passed", func() {
		AfterEach(func() {
			Expect(client.Stop()).To(Succeed())
		})

		It("logs at info level", func() {
			Eventually(client, "1s").Should(gbytes.Say(`"log_level":1`))
		})

		It("does not log at debug level", func() {
			Consistently(client, "1s").ShouldNot(gbytes.Say(`"log_level":0`))
		})

		Context("when I have a lot of API operations", func() {
			var handles []string

			JustBeforeEach(func() {
				handles = make([]string, 5)

				for i := 0; i < 5; i++ {

					container, err := client.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())

					handles = append(handles, container.Handle())
					// create process
					process, err := container.Run(garden.ProcessSpec{Path: "/bin/ls"}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())
					_, err = process.Wait()
					Expect(err).NotTo(HaveOccurred())

					// bulkinfo
					_, err = client.BulkInfo(handles)
					Expect(err).NotTo(HaveOccurred())
					_, err = client.BulkMetrics(handles)
					Expect(err).NotTo(HaveOccurred())
				}

				// destroy containers
				Expect(client.DestroyContainers()).To(Succeed())
			})

			AfterEach(func() {
				Expect(client.DestroyAndStop()).To(Succeed())
			})

			It("doesn't log too many messages", func() {
				outLines := strings.Split(string(client.Buffer().Contents()), "\n")
				Expect(len(outLines)).To(BeNumerically("<", 300))
			})
		})
	})

	Context("when a nonsense log-level is passed", func() {
		BeforeEach(func() {
			args = []string{"--log-level", "nonsense-log-level"}
		})

		It("exits with a code of 1", func() {
			Eventually(client, "1s").Should(gexec.Exit(1))
		})
	})
})
