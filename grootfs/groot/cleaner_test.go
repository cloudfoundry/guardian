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
		fakeGarbageCollector *grootfakes.FakeGarbageCollector
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
		fakeGarbageCollector = new(grootfakes.FakeGarbageCollector)

		cleaner = groot.IamCleaner(fakeLocksmith, fakeGarbageCollector)
		logger = lagertest.NewTestLogger("cleaner")
	})

	AfterEach(func() {
		Expect(os.Remove(lockFile.Name())).To(Succeed())
	})

	Describe("Clean", func() {
		It("collects orphaned volumes", func() {
			Expect(cleaner.Clean(logger)).To(Succeed())
			Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(1))
		})

		Context("when garbage collecting fails", func() {
			BeforeEach(func() {
				fakeGarbageCollector.CollectReturns(errors.New("failed to collect unused bits"))
			})

			It("returns an error", func() {
				Expect(cleaner.Clean(logger)).To(MatchError(ContainSubstring("failed to collect unused bits")))
			})
		})

		It("acquires the global lock", func() {
			Expect(cleaner.Clean(logger)).To(Succeed())

			Expect(fakeLocksmith.LockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.LockArgsForCall(0)).To(Equal(groot.GLOBAL_LOCK_KEY))
		})

		Context("when acquiring the lock fails", func() {
			BeforeEach(func() {
				fakeLocksmith.LockReturns(nil, errors.New("failed to acquire lock"))
			})

			It("returns the error", func() {
				err := cleaner.Clean(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to acquire lock")))
			})

			It("does not collect the garbage", func() {
				Expect(cleaner.Clean(logger)).NotTo(Succeed())
				Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(0))
			})
		})

		It("releases the global lock", func() {
			Expect(cleaner.Clean(logger)).To(Succeed())

			Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.UnlockArgsForCall(0)).To(Equal(lockFile))
		})
	})
})
