package groot_test

import (
	"errors"
	"io/ioutil"
	"os"

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

		cleaner *groot.Cleaner
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
		It("calls the garbage collector", func() {
			_, err := cleaner.Clean(logger, 0, []string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(1))
		})

		Context("when garbage collecting fails", func() {
			BeforeEach(func() {
				fakeGarbageCollector.CollectReturns(errors.New("failed to collect unused bits"))
			})

			It("returns an error", func() {
				_, err := cleaner.Clean(logger, 0, []string{})
				Expect(err).To(MatchError(ContainSubstring("failed to collect unused bits")))
			})
		})

		It("acquires the global lock", func() {
			_, err := cleaner.Clean(logger, 0, []string{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.LockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.LockArgsForCall(0)).To(Equal(groot.GlobalLockKey))
		})

		It("emits metrics for clean", func() {
			_, err := cleaner.Clean(logger, 0, []string{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsEmitter.TryEmitDurationCallCount()).To(Equal(1))
			_, name, duration := fakeMetricsEmitter.TryEmitDurationArgsForCall(0)
			Expect(name).To(Equal(groot.MetricImageCleanTime))
			Expect(duration).NotTo(BeZero())
		})

		Context("when acquiring the lock fails", func() {
			BeforeEach(func() {
				fakeLocksmith.LockReturns(nil, errors.New("failed to acquire lock"))
			})

			It("returns the error", func() {
				_, err := cleaner.Clean(logger, 0, []string{})
				Expect(err).To(MatchError(ContainSubstring("failed to acquire lock")))
			})

			It("does not collect the garbage", func() {
				_, err := cleaner.Clean(logger, 0, []string{})
				Expect(err).To(HaveOccurred())
				Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(0))
			})
		})

		It("releases the global lock", func() {
			_, err := cleaner.Clean(logger, 0, []string{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.UnlockArgsForCall(0)).To(Equal(lockFile))
		})

		Context("when a threshold is provided", func() {
			var threshold uint64

			BeforeEach(func() {
				threshold = 1000000
			})

			Context("when the store size is under the threshold", func() {
				BeforeEach(func() {
					fakeStoreMeasurer.MeasureStoreReturns(500000, nil)
				})

				It("does not remove anything", func() {
					_, err := cleaner.Clean(logger, threshold, []string{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(0))
				})

				It("does not acquire the lock", func() {
					_, err := cleaner.Clean(logger, threshold, []string{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeLocksmith.LockCallCount()).To(Equal(0))
				})

				It("sets noop to `true`", func() {
					noop, err := cleaner.Clean(logger, threshold, []string{})
					Expect(err).NotTo(HaveOccurred())
					Expect(noop).To(BeTrue())
				})
			})

			Context("when the store measurer fails", func() {
				BeforeEach(func() {
					fakeStoreMeasurer.MeasureStoreReturns(0, errors.New("failed to measure"))
				})

				It("returns the error", func() {
					_, err := cleaner.Clean(logger, threshold, []string{})
					Expect(err).To(MatchError(ContainSubstring("failed to measure")))
				})

				It("does not remove anything", func() {
					_, err := cleaner.Clean(logger, threshold, []string{})
					Expect(err).To(HaveOccurred())
					Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(0))
				})
			})

			Context("when the store size is over the threshold", func() {
				BeforeEach(func() {
					threshold = 1000000
					fakeStoreMeasurer.MeasureStoreReturns(1500000, nil)
				})

				It("calls the garbage collector", func() {
					_, err := cleaner.Clean(logger, threshold, []string{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(1))
				})
			})
		})
	})
})
