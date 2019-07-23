package execrunner_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("ProcessDirDepot", func() {
	var (
		bundlePath      string
		log             *lagertest.TestLogger
		bundleLookupper *depotfakes.FakeBundleLookupper
		processDepot    execrunner.ProcessDirDepot
	)

	BeforeEach(func() {
		bundleLookupper = new(depotfakes.FakeBundleLookupper)

		var err error
		bundlePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		bundleLookupper.LookupReturns(bundlePath, nil)

		log = lagertest.NewTestLogger("test")
		processDepot = execrunner.NewProcessDirDepot(bundleLookupper)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Describe("CreateProcessDir", func() {
		var (
			processPath string
			err         error
		)

		JustBeforeEach(func() {
			processPath, err = processDepot.CreateProcessDir(log, "sandbox-handle", "process-id")
		})

		It("creates the process directory in the depot", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(processPath).To(Equal(filepath.Join(bundlePath, "processes", "process-id")))
			Expect(processPath).To(BeADirectory())

			Expect(bundleLookupper.LookupCallCount()).To(Equal(1))
			_, actualSandboxHandle := bundleLookupper.LookupArgsForCall(0)
			Expect(actualSandboxHandle).To(Equal("sandbox-handle"))
		})

		When("looking up the bundle in the depot fails", func() {
			BeforeEach(func() {
				bundleLookupper.LookupReturns("", errors.New("lookup-error"))
			})

			It("fails", func() {
				Expect(err).To(MatchError("lookup-error"))
			})
		})
	})

	Describe("LookupProcessDir", func() {
		var processDir string
		var err error

		JustBeforeEach(func() {
			processDir, err = processDepot.LookupProcessDir(log, "sandbox-handle", "the-process")
		})

		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(bundlePath, "processes", "the-process"), 0755)).To(Succeed())
		})

		It("returns the process dir", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(processDir).To(Equal(filepath.Join(bundlePath, "processes", "the-process")))
		})

		When("looking up the bundle path fails", func() {
			BeforeEach(func() {
				bundleLookupper.LookupReturns("", errors.New("lookup-error"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("lookup-error"))
			})
		})

		When("the processes folder does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(bundlePath, "processes"))).To(Succeed())
			})

			It("returns an process not found error", func() {
				Expect(err).To(MatchError("process the-process not found"))
			})
		})
	})

	Describe("ListProcessDirs", func() {
		var processDirs []string
		var err error

		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(bundlePath, "processes", "one"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(bundlePath, "processes", "two"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(bundlePath, "processes", "three"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "processes", "not-a-dir"), []byte{}, 0755)).To(Succeed())
		})

		JustBeforeEach(func() {
			processDirs, err = processDepot.ListProcessDirs(log, "sandbox-handle")
		})

		It("returns the list of process dirs for a container", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(processDirs).To(ConsistOf(
				filepath.Join(bundlePath, "processes", "one"),
				filepath.Join(bundlePath, "processes", "two"),
				filepath.Join(bundlePath, "processes", "three"),
			))
		})

		When("looking up the bundle path fails", func() {
			BeforeEach(func() {
				bundleLookupper.LookupReturns("", errors.New("lookup-error"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("lookup-error"))
			})
		})

		When("the processes folder does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(bundlePath, "processes"))).To(Succeed())
			})

			It("returns an empty list", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(processDirs).To(BeEmpty())
			})
		})
	})
})
