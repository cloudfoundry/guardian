package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var dadooBinPath string

func TestDadoo(t *testing.T) {
	RegisterFailHandler(Fail)

	skip := os.Getenv("GARDEN_TEST_ROOTFS") == ""

	SynchronizedBeforeSuite(func() []byte {
		var err error
		bins := make(map[string]string)

		if skip {
			return nil
		}

		bins["dadoo_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/dadoo")
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

		dadooBinPath = bins["dadoo_bin_path"]
	})

	SynchronizedAfterSuite(func() {}, func() {
		mountsFileContent, err := ioutil.ReadFile("/proc/self/mounts")
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(string(mountsFileContent), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}

			fields := strings.Fields(line)
			if fields[2] == "cgroup" {
				Expect(syscall.Unmount(fields[1], 0)).To(Succeed())
			}
		}
	})

	BeforeEach(func() {
		if skip {
			Skip("dadoo requires linux")
		}
	})

	RunSpecs(t, "Dadoo Suite")
}
