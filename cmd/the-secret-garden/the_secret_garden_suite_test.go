package main_test

import (
	"encoding/json"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var theSecretGardenBin string

func TestThesecretGarden(t *testing.T) {
	RegisterFailHandler(Fail)
	skip := os.Getenv("GARDEN_TEST_ROOTFS") == ""

	SynchronizedBeforeSuite(func() []byte {
		var err error
		bins := make(map[string]string)

		if skip {
			return nil
		}

		bins["the_secret_garden"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/the-secret-garden")
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

		theSecretGardenBin = bins["the_secret_garden"]
	})

	BeforeEach(func() {
		if skip {
			Skip("the-secret-garden requires linux")
		}
		SetDefaultEventuallyTimeout(5 * time.Second)
	})

	RunSpecs(t, "The Secret Garden Suite")
}
