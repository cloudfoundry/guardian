package gnatsdrunner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/cloudfoundry/gunk/diegonats"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

func StartGnatsd(natsPort int) (ifrit.Process, diegonats.NATSClient) {
	ginkgomonRunner := NewGnatsdTestRunner(natsPort)
	gnatsdProcess := ifrit.Envoke(ginkgomonRunner)

	natsClient := diegonats.NewClient()
	_, err := natsClient.Connect([]string{fmt.Sprintf("nats://127.0.0.1:%d", natsPort)})
	Expect(err).ShouldNot(HaveOccurred())

	return gnatsdProcess, natsClient
}

func NewGnatsdTestRunner(natsPort int) *ginkgomon.Runner {
	gnatsdPath, err := exec.LookPath("gnatsd")
	if err != nil {
		fmt.Println("You need gnatsd installed!")
		os.Exit(1)
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:              "gnatsd",
		AnsiColorCode:     "99m",
		StartCheck:        "gnatsd is ready",
		StartCheckTimeout: 5 * time.Second,
		Command: exec.Command(
			gnatsdPath,
			"-p", strconv.Itoa(natsPort),
		),
	})
}
