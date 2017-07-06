package gqt_test

import (
	"io"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Fuse", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
	)

	BeforeEach(func() {
		fuseRootfs := os.Getenv("GARDEN_FUSE_TEST_ROOTFS")
		if fuseRootfs == "" {
			Skip("GARDEN_FUSE_TEST_ROOTFS not defined, skipping")
		}

		var err error
		client = runner.Start(config)
		container, err = client.Create(garden.ContainerSpec{
			RootFSPath: fuseRootfs,
			Privileged: true,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("can mount fuse in the container", func() {
		stdout := gbytes.NewBuffer()
		sess, err := container.Run(garden.ProcessSpec{
			Path: "sh",
			Args: []string{
				"-c", `mkdir -p /tmp/fusetest && /usr/bin/hellofs /tmp/fusetest; cat /tmp/fusetest/hello`,
			},
		}, garden.ProcessIO{Stdout: io.MultiWriter(stdout, GinkgoWriter), Stderr: GinkgoWriter})
		Expect(err).NotTo(HaveOccurred())
		Expect(sess.Wait()).To(Equal(0))
		Expect(stdout).To(gbytes.Say("Hello World!"))
	})
})
