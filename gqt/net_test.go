package gqt_test

import (
	"fmt"
	"io"
	"net"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Net", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
	)

	BeforeEach(func() {
		var err error

		client = startGarden()

		container, err = client.Create(garden.ContainerSpec{
			Network: "192.168.12.0/24",
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		client.Destroy(container.Handle())
	})

	It("should have a loopback interface", func() {
		buffer := gbytes.NewBuffer()
		proc, err := container.Run(
			garden.ProcessSpec{
				Path: "ifconfig",
				User: "root",
			}, garden.ProcessIO{Stdout: io.MultiWriter(GinkgoWriter, buffer), Stderr: GinkgoWriter},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(proc.Wait()).To(Equal(0))

		Expect(buffer).To(gbytes.Say("lo"))
	})

	It("should have a (dynamically assigned) IP address", func() {
		buffer := gbytes.NewBuffer()
		proc, err := container.Run(
			garden.ProcessSpec{
				Path: "ifconfig",
				User: "root",
			}, garden.ProcessIO{Stdout: buffer},
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(proc.Wait()).To(Equal(0))

		Expect(buffer).To(gbytes.Say("192.168.12.2"))
	})

	It("should be pingable", func() {
		out, err := exec.Command("/bin/ping", "-c 2", "192.168.12.2").Output()
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring(" 0% packet loss"))
	})

	Context("a second container", func() {
		var originContainer garden.Container

		BeforeEach(func() {
			var err error
			originContainer = container
			container, err = client.Create(garden.ContainerSpec{
				Network: "192.168.12.0/24",
			})

			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Destroy(originContainer.Handle())).To(Succeed())
		})

		It("should have the next IP address", func() {
			buffer := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "ifconfig",
					User: "root",
				}, garden.ProcessIO{Stdout: buffer},
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(proc.Wait()).To(Equal(0))

			Expect(buffer).To(gbytes.Say("192.168.12.3"))
		})

		It("should be pingable", func() {
			out, err := exec.Command("/bin/ping", "-c 2", "192.168.12.2").Output()
			Expect(out).To(ContainSubstring(" 0% packet loss"))
			Expect(err).ToNot(HaveOccurred())

			out, err = exec.Command("/bin/ping", "-c 2", "192.168.12.3").Output()
			Expect(out).To(ContainSubstring(" 0% packet loss"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should access internet", func() {
			ips, err := net.LookupIP("www.example.com")
			Expect(err).ToNot(HaveOccurred())

			Expect(checkConnection(container, ips[0].String(), 80)).To(Succeed())
		})
	})
})

func checkConnection(container garden.Container, ip string, port int) error {
	process, err := container.Run(garden.ProcessSpec{
		User: "alice",
		Path: "sh",
		Args: []string{"-c", fmt.Sprintf("echo hello | nc -w1 %s %d", ip, port)},
	}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
	if err != nil {
		return err
	}

	exitCode, err := process.Wait()
	if err != nil {
		return err
	}

	if exitCode == 0 {
		return nil
	} else {
		return fmt.Errorf("Request failed. Process exited with code %d", exitCode)
	}
}
