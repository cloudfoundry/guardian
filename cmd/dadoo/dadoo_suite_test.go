//go:build !windows

package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

var (
	dadooBinPath    string
	fakeRuncBinPath string
	cgroupsRoot     string
)

func TestDadoo(t *testing.T) {
	RegisterFailHandler(Fail)

	skip := os.Getenv("GARDEN_TEST_ROOTFS") == ""

	SynchronizedBeforeSuite(func() []byte {
		var err error
		bins := make(map[string]string)

		if skip {
			return nil
		}

		cgroupsRoot = filepath.Join(os.TempDir(), "dadoo-cgroups")
		Expect(setupCgroups(cgroupsRoot)).To(Succeed())

		bins["dadoo_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/dadoo", "-mod=vendor")
		Expect(err).NotTo(HaveOccurred())

		bins["fakerunc_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/dadoo/fake_runc", "-mod=vendor")
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
		fakeRuncBinPath = bins["fakerunc_bin_path"]
	})

	SynchronizedAfterSuite(func() {}, func() {
		gexec.CleanupBuildArtifacts()
		mountsFileContent, err := os.ReadFile("/proc/self/mounts")
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

		if cgroups.IsCgroup2UnifiedMode() {
			Expect(syscall.Unmount(filepath.Join(cgroupsRoot, gardencgroups.Unified), 0)).To(Succeed())
		} else {
			Expect(syscall.Unmount(cgroupsRoot, 0)).To(Succeed())
		}
		Expect(os.Remove(cgroupsRoot)).To(Succeed())
	})

	BeforeEach(func() {
		if skip {
			Skip("dadoo requires linux")
		}
	})

	RunSpecs(t, "Dadoo Suite")
}
