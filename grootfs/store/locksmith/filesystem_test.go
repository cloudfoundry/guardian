package locksmith_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/locksmith"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filesystem", func() {
	var (
		fileSystemLock *locksmith.FileSystem
		metricsEmitter *grootfakes.FakeMetricsEmitter
		storePath      string
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "store")
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(storePath, "locks"), 0755)).To(Succeed())
		metricsEmitter = new(grootfakes.FakeMetricsEmitter)

		fileSystemLock = locksmith.NewFileSystem(storePath, metricsEmitter)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	It("blocks when locking the same key twice", func() {
		lockFd, err := fileSystemLock.Lock("key")
		Expect(err).NotTo(HaveOccurred())

		wentThrough := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			_, err := fileSystemLock.Lock("key")
			Expect(err).NotTo(HaveOccurred())

			close(wentThrough)
		}()

		Consistently(wentThrough).ShouldNot(BeClosed())
		Expect(fileSystemLock.Unlock(lockFd)).To(Succeed())
		Eventually(wentThrough).Should(BeClosed())
	})

	Describe("Lock", func() {
		It("creates the lock file in the lock path when it does not exist", func() {
			lockFile := filepath.Join(storePath, store.LocksDirName, "key.lock")

			Expect(lockFile).ToNot(BeAnExistingFile())
			_, err := fileSystemLock.Lock("key")
			Expect(err).NotTo(HaveOccurred())
			Expect(lockFile).To(BeAnExistingFile())
		})

		It("removes slashes(/) from key name", func() {
			lockFile := filepath.Join(storePath, store.LocksDirName, "/tmpkey.lock")

			Expect(lockFile).ToNot(BeAnExistingFile())
			_, err := fileSystemLock.Lock("/tmp/key")
			Expect(err).NotTo(HaveOccurred())
			Expect(lockFile).To(BeAnExistingFile())
		})

		It("emits the locking time metric", func() {
			startTime := time.Now()
			_ = filepath.Join(storePath, store.LocksDirName, "key.lock")
			_, err := fileSystemLock.Lock("key")
			Expect(err).NotTo(HaveOccurred())

			Expect(metricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
			_, metricName, fromTime := metricsEmitter.TryEmitDurationFromArgsForCall(0)

			Expect(metricName).To(Equal(locksmith.MetricsLockingTime))
			Expect(fromTime.Unix()).To(BeNumerically("~", startTime.Unix(), 1))
		})

		Context("when creating the lock file fails", func() {
			BeforeEach(func() {
				storePath = "/not/real"
				fileSystemLock = locksmith.NewFileSystem(storePath, metricsEmitter)
			})

			It("returns an error", func() {
				lockFile := filepath.Join(storePath, "key")

				_, err := fileSystemLock.Lock("key")
				Expect(err).To(MatchError(ContainSubstring("creating lock file for key `key`:")))
				Expect(lockFile).ToNot(BeAnExistingFile())
			})
		})

		Context("when locking the file fails", func() {
			BeforeEach(func() {
				locksmith.FlockSyscall = func(_ int, _ int) error {
					return errors.New("failed to lock file")
				}
			})

			It("returns an error", func() {
				_, err := fileSystemLock.Lock("key")
				Expect(err).To(MatchError(ContainSubstring("failed to lock file")))
			})
		})
	})

	Context("Unlock", func() {
		Context("when unlocking a file descriptor fails", func() {
			var lockFile *os.File

			BeforeEach(func() {
				lockFile = os.NewFile(uintptr(12), "lockFile")
				locksmith.FlockSyscall = func(_ int, _ int) error {
					return errors.New("failed to unlock file")
				}
			})

			It("returns an error", func() {
				Expect(fileSystemLock.Unlock(lockFile)).To(
					MatchError(ContainSubstring("failed to unlock file")),
				)
			})
		})
	})
})
