package groot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cleaner", func() {
	var (
		fakeLocksmith        *grootfakes.FakeLocksmith
		fakeStoreMeasurer    *grootfakes.FakeStoreMeasurer
		fakeGarbageCollector *grootfakes.FakeGarbageCollector
		fakeMetricsEmitter   *grootfakes.FakeMetricsEmitter
		lockFile             *os.File

		cleaner groot.Cleaner
		logger  lager.Logger
	)

	BeforeEach(func() {
		var err error
		fakeLocksmith = new(grootfakes.FakeLocksmith)
		lockFile, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		fakeLocksmith.LockReturns(lockFile, nil)

		fakeStoreMeasurer = new(grootfakes.FakeStoreMeasurer)
		fakeGarbageCollector = new(grootfakes.FakeGarbageCollector)
		fakeMetricsEmitter = new(grootfakes.FakeMetricsEmitter)

		cleaner = groot.IamCleaner(fakeLocksmith, fakeStoreMeasurer,
			fakeGarbageCollector, fakeMetricsEmitter)
		logger = lagertest.NewTestLogger("cleaner")
	})

	AfterEach(func() {
		Expect(os.Remove(lockFile.Name())).To(Succeed())
	})

	Describe("Clean", func() {
		It("calls the garbage collector to gather a list of unused volumes", func() {
			_, err := cleaner.Clean(logger, 0, []string{"preserve"})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeGarbageCollector.UnusedVolumesCallCount()).To(Equal(1))
			_, actualChainIDsToPreserve := fakeGarbageCollector.UnusedVolumesArgsForCall(0)
			Expect(actualChainIDsToPreserve).To(ConsistOf("preserve"))
		})

		It("calls the garbage collector to mark unused volumes", func() {
			_, err := cleaner.Clean(logger, 0, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeGarbageCollector.MarkUnusedCallCount()).To(Equal(1))
		})

		It("calls the garbage collector to collect", func() {
			_, err := cleaner.Clean(logger, 0, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(1))
		})

		Context("when garbage collecting fails", func() {
			BeforeEach(func() {
				fakeGarbageCollector.CollectReturns(errors.New("failed to collect unused bits"))
			})

			It("returns an error", func() {
				_, err := cleaner.Clean(logger, 0, nil)
				Expect(err).To(MatchError(ContainSubstring("failed to collect unused bits")))
			})
		})

		It("emits metrics for clean duration", func() {
			_, err := cleaner.Clean(logger, 0, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
			_, name, start := fakeMetricsEmitter.TryEmitDurationFromArgsForCall(0)
			Expect(name).To(Equal(groot.MetricImageCleanTime))
			Expect(start).NotTo(BeZero())
		})

		It("acquires the global lock", func() {
			_, err := cleaner.Clean(logger, 0, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.LockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.LockArgsForCall(0)).To(Equal(groot.GlobalLockKey))
		})

		It("releases the global lock", func() {
			_, err := cleaner.Clean(logger, 0, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.UnlockArgsForCall(0)).To(Equal(lockFile))
		})

		It("releases the lock between marking unsused and collecting garbage", func() {
			var unLockTime, markTime, collectTime time.Time
			fakeLocksmith.UnlockStub = func(_ *os.File) error {
				unLockTime = time.Now()
				return nil
			}

			fakeGarbageCollector.MarkUnusedStub = func(_ lager.Logger, _ []string) error {
				markTime = time.Now()
				return nil
			}

			fakeGarbageCollector.CollectStub = func(_ lager.Logger) error {
				collectTime = time.Now()
				return nil
			}

			_, err := cleaner.Clean(logger, 0, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(markTime.UnixNano()).To(BeNumerically("<", unLockTime.UnixNano()))
			Expect(unLockTime.UnixNano()).To(BeNumerically("<", collectTime.UnixNano()))
		})

		Context("when marking unused volumes fails", func() {
			BeforeEach(func() {
				fakeGarbageCollector.MarkUnusedReturns(errors.New("Failed to mark!"))
			})

			It("still collects the garbage", func() {
				_, err := cleaner.Clean(logger, 0, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(1))
			})

			It("releases the global lock", func() {
				_, err := cleaner.Clean(logger, 0, nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
			})
		})

		Context("when acquiring the lock fails", func() {
			BeforeEach(func() {
				fakeLocksmith.LockReturns(nil, errors.New("failed to acquire lock"))
			})

			It("returns the error", func() {
				_, err := cleaner.Clean(logger, 0, nil)
				Expect(err).To(MatchError(ContainSubstring("failed to acquire lock")))
			})

			It("does not collect the garbage", func() {
				_, err := cleaner.Clean(logger, 0, nil)
				Expect(err).To(HaveOccurred())
				Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(0))
			})
		})

		Context("when a threshold is provided", func() {
			var threshold int64

			BeforeEach(func() {
				threshold = 1000000
			})

			Context("when the store size under the threshold", func() {
				BeforeEach(func() {
					fakeGarbageCollector.UnusedVolumesReturns([]string{"layerVolume", "localVolume-1234"}, nil)
					fakeStoreMeasurer.CacheUsageReturns(500000)
				})

				It("does not remove anything", func() {
					_, err := cleaner.Clean(logger, threshold, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(0))
				})

				It("does not acquire the lock", func() {
					_, err := cleaner.Clean(logger, threshold, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeLocksmith.LockCallCount()).To(Equal(0))
				})

				It("sets noop to `true`", func() {
					noop, err := cleaner.Clean(logger, threshold, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(noop).To(BeTrue())
				})
			})

			Context("when the store size is greater than the threshold", func() {
				BeforeEach(func() {
					threshold = 1000000
					fakeStoreMeasurer.UsageReturns(1500000, nil)
				})

				It("calls the garbage collector", func() {
					_, err := cleaner.Clean(logger, threshold, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(1))
				})
			})

			Context("when the cache bytes is negative", func() {
				BeforeEach(func() {
					threshold = -120
				})

				It("indicates a no-op and returns an error", func() {
					noop, err := cleaner.Clean(logger, threshold, nil)
					Expect(noop).To(BeTrue())
					Expect(err).To(MatchError("Threshold must be greater than 0"))
				})
			})
		})
	})
})
