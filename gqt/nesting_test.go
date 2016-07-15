package gqt_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/guardian/gqt/runner"
)

var _ = Describe("When nested", func() {
	nestedRootfsPath := os.Getenv("GARDEN_NESTABLE_TEST_ROOTFS")
	var client *runner.RunningGarden
	BeforeEach(func() {
		client = startGarden()
	})

	startNestedGarden := func() (garden.Container, string) {
		absoluteGardenPath, err := filepath.Abs(gardenBin)
		Expect(err).ToNot(HaveOccurred())

		absoluteRuncPath, err := filepath.Abs("/usr/local/bin/runc")
		Expect(err).ToNot(HaveOccurred())

		absoluteIODaemonPath, err := filepath.Abs(iodaemonBin)
		Expect(err).ToNot(HaveOccurred())

		absoluteDadooPath, err := filepath.Abs(dadooBin)
		Expect(err).ToNot(HaveOccurred())

		absoluteInitPath, err := filepath.Abs(initBin)
		Expect(err).ToNot(HaveOccurred())

		absoluteTarPath, err := filepath.Abs(runner.TarBin)
		Expect(err).ToNot(HaveOccurred())

		absoluteNstarPath, err := filepath.Abs(nstarBin)
		Expect(err).ToNot(HaveOccurred())

		container, err := client.Create(garden.ContainerSpec{
			RootFSPath: nestedRootfsPath,
			// only privileged containers support nesting
			Privileged: true,
			BindMounts: []garden.BindMount{
				{
					SrcPath: filepath.Dir(absoluteGardenPath),
					DstPath: "/tmp/bin/",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: runner.RootFSPath,
					DstPath: "/tmp/rootfs",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: filepath.Dir(absoluteRuncPath),
					DstPath: "/tmp/runc/",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: filepath.Dir(absoluteIODaemonPath),
					DstPath: "/tmp/iodaemon/",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: filepath.Dir(absoluteDadooPath),
					DstPath: "/tmp/dadoo/",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: filepath.Dir(absoluteInitPath),
					DstPath: "/tmp/init/",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: filepath.Dir(absoluteTarPath),
					DstPath: "/tmp/tar/",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: filepath.Dir(absoluteNstarPath),
					DstPath: "/tmp/nstar/",
					Mode:    garden.BindMountModeRO,
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		nestedServerOutput := gbytes.NewBuffer()

		_, err = container.Run(garden.ProcessSpec{
			Path: "sh",
			User: "root",
			Dir:  "/tmp",
			Args: []string{
				"-c",
				fmt.Sprintf(`
				set -e

				tmpdir=/tmp/dir
				rm -fr $tmpdir
				mkdir $tmpdir
				mount -t tmpfs none $tmpdir
				echo "{}" > /tmp/network.props

				mkdir $tmpdir/depot
				mkdir $tmpdir/snapshots
				mkdir $tmpdir/state
				mkdir $tmpdir/graph

				/tmp/bin/guardian \
					--default-rootfs /tmp/rootfs \
					--depot $tmpdir/depot \
					--graph $tmpdir/graph \
					--tag n \
					--bind-socket tcp \
					--bind-ip 0.0.0.0 \
					--bind-port 7778 \
					--network-pool 10.254.6.0/22 \
					--runc-bin /tmp/runc/runc \
					--init-bin /tmp/init/init \
					--iodaemon-bin /tmp/iodaemon/iodaemon \
					--dadoo-bin /tmp/dadoo/dadoo \
					--nstar-bin /tmp/nstar/nstar \
					--port-pool-properties-path /tmp/network.props \
					--tar-bin /tmp/tar/tar \
					--port-pool-start 30000
				`),
			},
		}, garden.ProcessIO{
			Stdout: io.MultiWriter(nestedServerOutput, gexec.NewPrefixedWriter("\x1b[32m[o]\x1b[34m[nested-garden-runc]\x1b[0m ", GinkgoWriter)),
			Stderr: gexec.NewPrefixedWriter("\x1b[91m[e]\x1b[34m[nested-garden-runc]\x1b[0m ", GinkgoWriter),
		})

		info, err := container.Info()
		Expect(err).ToNot(HaveOccurred())

		nestedGardenAddress := fmt.Sprintf("%s:7778", info.ContainerIP)
		Eventually(nestedServerOutput, "60s").Should(gbytes.Say("guardian.started"))

		return container, nestedGardenAddress
	}

	It("can start a nested garden server and run a container inside it", func() {
		container, nestedGardenAddress := startNestedGarden()
		defer func() {
			Expect(client.Destroy(container.Handle())).To(Succeed())
		}()

		nestedClient := gclient.New(gconn.New("tcp", nestedGardenAddress))
		nestedContainer, err := nestedClient.Create(garden.ContainerSpec{})
		Expect(err).ToNot(HaveOccurred())

		nestedOutput := gbytes.NewBuffer()
		_, err = nestedContainer.Run(garden.ProcessSpec{
			User: "root",
			Path: "/bin/echo",
			Args: []string{
				"I am nested!",
			},
		}, garden.ProcessIO{Stdout: nestedOutput, Stderr: nestedOutput})
		Expect(err).ToNot(HaveOccurred())

		Eventually(nestedOutput, "60s").Should(gbytes.Say("I am nested!"))
	})
})
