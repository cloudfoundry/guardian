package gqt_test

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Networking Uniqueness", func() {
	var (
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		config.LogLevel = "error"
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("should not allocate duplicate subnets", func() {
		for i := 0; i < 4; i++ {
			go func() {
				defer GinkgoRecover()

				create(client, 5)
			}()
		}

		Eventually(numContainers(client), "120s").Should(Equal(20))
		Expect(numBridges()).To(Equal(20), runCommand(exec.Command("ifconfig")))
	})
})

func create(client *runner.RunningGarden, n int) {
	for i := 0; i < n; i++ {
		time.Sleep(randomSleepDuration())

		id, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())
		randomID := "net-uniq-" + id.String()
		_, err = client.Create(garden.ContainerSpec{Handle: randomID})
		Expect(err).NotTo(HaveOccurred())

	}
}

func randomSleepDuration() time.Duration {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	duration := time.Duration(50+random.Intn(500)) * time.Millisecond
	return duration
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
