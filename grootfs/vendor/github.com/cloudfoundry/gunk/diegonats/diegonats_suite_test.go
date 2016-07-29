package diegonats_test

import (
	"testing"

	"github.com/cloudfoundry/gunk/diegonats/gnatsdrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

func TestDiegoNATS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Diego NATS Suite")
}

var natsPort int
var gnatsdProcess ifrit.Process

var _ = BeforeSuite(func() {
	natsPort = 4001 + GinkgoParallelNode()
})

var _ = AfterSuite(func() {
})

func startNATS() {
	gnatsdProcess = ginkgomon.Invoke(gnatsdrunner.NewGnatsdTestRunner(natsPort))
}

func stopNATS() {
	ginkgomon.Kill(gnatsdProcess)
}
