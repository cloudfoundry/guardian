package gqt_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Run", func() {
	var client *runner.RunningGarden

	FIt("should run a process and return the exit code", func() {
		client = startGarden()
		container, err := client.Create(garden.ContainerSpec{
			Handle: "fred",
		})
		Expect(err).NotTo(HaveOccurred())

		proc, err := container.Run(garden.ProcessSpec{
			Path: "sh",
			Args: []string{"-c", "exit 12"},
		}, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())

		exitCode, err := proc.Wait()
		Expect(err).NotTo(HaveOccurred())
		Expect(exitCode).To(Equal(12))
	})
})
