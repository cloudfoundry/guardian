package gqt_test

import (
	"fmt"
	"net"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Networking Uniqueness", func() {
	var (
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("should not allocate duplicate subnets", func() {
		for i := 0; i < 10; i++ {
			go func() {
				defer GinkgoRecover()

				create(client, 5)
			}()
		}

		Eventually(numContainers(client), "20s").Should(Equal(50))
		Expect(numBridges()).To(Equal(50))
	})
})

func create(client *runner.RunningGarden, n int) {
	for i := 0; i < n; i++ {
		_, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
	}
}

func numContainers(client *runner.RunningGarden) func() int {
	return func() int {
		containers, err := client.Containers(nil)
		Expect(err).NotTo(HaveOccurred())
		return len(containers)
	}
}

func numBridges() int {
	intfs, err := net.Interfaces()
	Expect(err).NotTo(HaveOccurred())

	bridgeCount := 0

	for _, intf := range intfs {
		if strings.Contains(intf.Name, fmt.Sprintf("w%dbrdg", GinkgoParallelNode())) {
			bridgeCount++
		}
	}

	return bridgeCount
}
