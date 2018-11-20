package gqt_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"sync"
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
		skipIfDev()
		routines := 25
		containersPerRoutine := 10
		allContainerInfos := []garden.ContainerInfo{}
		var mutex = &sync.Mutex{}

		wg := &sync.WaitGroup{}
		wg.Add(routines)
		for i := 0; i < routines; i++ {
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				containerInfos := create(client, containersPerRoutine)

				mutex.Lock()
				defer mutex.Unlock()
				allContainerInfos = append(allContainerInfos, containerInfos...)
			}()
		}

		wg.Wait()
		Expect(numContainers(client)).To(Equal(routines * containersPerRoutine))
		Expect(numBridges()).To(Equal(routines*containersPerRoutine), diagnose(allContainerInfos))
	})
})

func diagnose(containerInfos []garden.ContainerInfo) string {
	ifconfigBytes, err := exec.Command("ifconfig").CombinedOutput()
	interfaceInfo := string(ifconfigBytes)
	if err != nil {
		interfaceInfo += fmt.Sprintf("\nifconfig error: %v\n", err)
	}

	var allContainersInfo string
	bytes, err := json.Marshal(containerInfos)
	if err != nil {
		allContainersInfo = fmt.Sprintf("Could not marshal containers due to %v; raw containers data:\n %#v", err, containerInfos)
	} else {
		allContainersInfo = string(bytes)
	}

	return fmt.Sprintf("%s\n%s\n", interfaceInfo, allContainersInfo)
}

func create(client *runner.RunningGarden, n int) []garden.ContainerInfo {
	containerInfos := []garden.ContainerInfo{}
	for i := 0; i < n; i++ {
		time.Sleep(randomSleepDuration())

		id, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())
		randomID := "net-uniq-" + id.String()
		container, err := client.Create(garden.ContainerSpec{Handle: randomID})
		Expect(err).NotTo(HaveOccurred())
		containerInfo, err := container.Info()
		Expect(err).NotTo(HaveOccurred())
		containerInfos = append(containerInfos, containerInfo)

	}
	return containerInfos
}

func randomSleepDuration() time.Duration {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	duration := time.Duration(50+random.Intn(500)) * time.Millisecond
	return duration
}

func numContainers(client *runner.RunningGarden) int {
	containers, err := client.Containers(nil)
	Expect(err).NotTo(HaveOccurred())
	return len(containers)
}

func numBridges() int {
	intfs, err := net.Interfaces()
	Expect(err).NotTo(HaveOccurred())

	bridgeCount := 0

	for _, intf := range intfs {
		if strings.Contains(intf.Name, fmt.Sprintf("w%dbrdg-0afe", GinkgoParallelNode())) {
			bridgeCount++
		}
	}

	return bridgeCount
}
