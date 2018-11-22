package gqt_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
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

		ctrl := make(chan struct{})

		go func() {
			defer GinkgoRecover()

			for {
				out, err := exec.Command("/bin/sh", "-c", "iptables-save | grep DNAT").Output()
				if err != nil {
					continue
				}

				ipInfos := parseIPTablesOutput(out)
				if len(ipInfos) == 0 {
					continue
				}

				ipSet := map[string]struct{}{}

				for _, info := range ipInfos {
					ipSet[info.Ip] = struct{}{}
				}

				Expect(ipSet).To(HaveLen(len(ipInfos)), diagnose(ipInfos, out))

				if haveToBail(ctrl) {
					break
				}
			}
		}()

		routines := 25
		containersPerRoutine := 10
		wg := &sync.WaitGroup{}
		wg.Add(routines)
		for i := 0; i < routines; i++ {
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				createAndDestroy(client, containersPerRoutine)

				if haveToBail(ctrl) {
					return
				}
			}()
		}

		wg.Wait()
		close(ctrl)
	})
})

func createAndDestroy(client *runner.RunningGarden, n int) {
	for i := 0; i < n; i++ {
		time.Sleep(randomSleepDuration())

		id, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())
		randomID := "net-uniq-" + id.String()
		container, err := client.Create(garden.ContainerSpec{
			Handle: randomID,
			NetIn:  []garden.NetIn{garden.NetIn{HostPort: 8080}},
		})
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(randomSleepDuration())

		err = client.Destroy(container.Handle())
		Expect(err).NotTo(HaveOccurred())
	}
}

func haveToBail(ctrl chan struct{}) bool {
	select {
	case <-ctrl:
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

func parseIPTablesOutput(output []byte) []containerIPInfo {
	ips := []containerIPInfo{}

	outString := string(bytes.TrimSuffix(output, []byte{'\n'}))

	portMappings := strings.Split(outString, "\n")

	for _, portMapping := range portMappings {
		fields := strings.Split(portMapping, " ")
		ip := strings.Split(fields[17], ":")[0]
		handle := fields[13]
		ips = append(ips, containerIPInfo{handle, ip})
	}

	return ips
}

func diagnose(infos []containerIPInfo, iptablesOut []byte) string {
	containerInfo := fmt.Sprintf("%#v", infos)
	infoJson, err := json.Marshal(infos)
	if err == nil {
		containerInfo = string(infoJson)
	}
	return fmt.Sprintf("IpTables Port Mapping Ruels:\n%s\nContainers Info:\n%s\n", iptablesOut, containerInfo)
}

func randomSleepDuration() time.Duration {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	duration := time.Duration(50+random.Intn(500)) * time.Millisecond
	return duration
}

type containerIPInfo struct {
	Handle string
	Ip     string
}
