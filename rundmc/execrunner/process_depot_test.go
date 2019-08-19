package execrunner_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/lager"
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

	Describe("CreatedTime", func() {
		var (
			processID        string
			pidFile          *os.File
			pidModTime       time.Time
			createdTime      time.Time
			err              error
			firstBundlePath  string
			secondBundlePath string
			thirdBundlePath  string
		)

		BeforeEach(func() {
			processID = "process-id"
			pidModTime = time.Date(2019, 8, 15, 14, 27, 32, 0, time.UTC)

			var err error
			firstBundlePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			secondBundlePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			thirdBundlePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			bundleLookupper.LookupStub = func(_ lager.Logger, sandboxHandle string) (string, error) {
				switch sandboxHandle {
				case "first":
					return firstBundlePath, nil
				case "second":
					return secondBundlePath, nil
				case "third":
					return thirdBundlePath, nil
				default:
					return "", errors.New("not found")
				}
			}

			bundleLookupper.HandlesReturns([]string{"first", "second", "third"}, nil)

			err = os.MkdirAll(filepath.Join(secondBundlePath, "processes", "process-id"), 0755)
			Expect(err).NotTo(HaveOccurred())
			pidFile, err = os.Create(filepath.Join(secondBundlePath, "processes", "process-id", "pidfile"))
			Expect(err).NotTo(HaveOccurred())
			err = os.Chtimes(pidFile.Name(), pidModTime, pidModTime)
			Expect(err).NotTo(HaveOccurred())
			err = os.MkdirAll(filepath.Join(thirdBundlePath, "processes", "another-process-id"), 0755)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(pidFile.Close()).To(Succeed())
			Expect(os.RemoveAll(firstBundlePath)).To(Succeed())
			Expect(os.RemoveAll(secondBundlePath)).To(Succeed())
			Expect(os.RemoveAll(thirdBundlePath)).To(Succeed())
		})

		JustBeforeEach(func() {
			createdTime, err = processDepot.CreatedTime(log, processID)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the approximate creation time of the pea", func() {
			Expect(createdTime).To(BeTemporally("==", pidModTime))
		})

		Context("when the bundle is not there", func() {
			BeforeEach(func() {
				processID = "unexistant-process-id"
			})

			It("fails", func() {
				_, err := processDepot.CreatedTime(log, "unexistant-process-id")
				Expect(err).To(MatchError("process unexistant-process-id not found"))
			})
		})

		Context("when the process pidfile is not there", func() {
			BeforeEach(func() {
				processID = "another-process-id"
			})

			It("fails", func() {
				_, err = processDepot.CreatedTime(log, "another-process-id")
				Expect(err).To(MatchError(ContainSubstring("process pidfile does not exist")))
			})
		})
	})
})
