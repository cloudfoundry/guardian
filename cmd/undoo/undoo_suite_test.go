package main_test

import (
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var undooBinPath string

func TestUndoo(t *testing.T) {
	RegisterFailHandler(Fail)

	skip := os.Getenv("GARDEN_TEST_ROOTFS") == ""

	SynchronizedBeforeSuite(func() []byte {
		var err error
		bins := make(map[string]string)

		if skip {
			return nil
		}

		bins["undoo_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/undoo")
		Expect(err).NotTo(HaveOccurred())

		data, err := json.Marshal(bins)
		Expect(err).NotTo(HaveOccurred())

		return data
	}, func(data []byte) {
		if skip {
			return
		}

		bins := make(map[string]string)
		Expect(json.Unmarshal(data, &bins)).To(Succeed())

		undooBinPath = bins["undoo_bin_path"]
	})

	BeforeEach(func() {
		if skip {
			Skip("undoo requires linux")
		}
	})

	RunSpecs(t, "Undoo Suite")
}
