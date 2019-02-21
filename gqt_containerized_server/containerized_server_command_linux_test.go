package gqt_containerized_server_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/properties"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("gdn containerized-server", func() {
	var (
		containerHandle string
		tmpDir          string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		config.PropertiesPath = path.Join(tmpDir, "props.json")
		client := runner.Start(config)
		container, err := client.Create(garden.ContainerSpec{
			Network: fmt.Sprintf("177.100.%d.0/24", GinkgoParallelNode()),
		})
		Expect(err).NotTo(HaveOccurred())

		containerHandle = container.Handle()

		Expect(client.Stop()).To(Succeed())
	})

	AfterEach(func() {
		client := runner.Start(config)
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		var err error
		containerizedServerArgs := []string{
			"containerized-server",
			"--depot", config.DepotDir,
			"--properties-path", config.PropertiesPath,
		}

		cmd := exec.Command(binaries.Gdn, containerizedServerArgs...)
		containerizedServerProcess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(containerizedServerProcess, 10*time.Second).Should(gexec.Exit(0))
	})

	It("cannot lookup the container anymore", func() {
		client := runner.Start(config)
		_, err := client.Lookup(containerHandle)
		Expect(err).To(Equal(garden.ContainerNotFoundError{Handle: containerHandle}))
		Expect(client.Stop()).To(Succeed())
	})

	It("cleans cleanedup containers properties", func() {
		manager, err := properties.Load(config.PropertiesPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager.All(containerHandle)).To(BeEmpty())
	})
})
