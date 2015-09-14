package process_tracker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"testing"

	"github.com/onsi/gomega/gexec"
)

var iodaemonBin string
var testPrintBin string

func TestProcess_tracker(t *testing.T) {
	var beforeSuite struct {
		IodaemonPath        string
		TestPrintSignalPath string
	}

	SynchronizedBeforeSuite(func() []byte {
		var err error
		beforeSuite.IodaemonPath, err = gexec.Build("github.com/cloudfoundry-incubator/guardian/rundmc/iodaemon/cmd/iodaemon")
		Expect(err).ToNot(HaveOccurred())

		b, err := json.Marshal(beforeSuite)
		Expect(err).ToNot(HaveOccurred())

		return b
	}, func(paths []byte) {
		err := json.Unmarshal(paths, &beforeSuite)
		Expect(err).ToNot(HaveOccurred())

		iodaemonBin = beforeSuite.IodaemonPath
		Expect(iodaemonBin).NotTo(BeEmpty())
	})

	SynchronizedAfterSuite(func() {
		//noop
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "Process Tracker Suite")
}
