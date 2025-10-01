package locksmith_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store/locksmith"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filesystem", func() {
	var (
		metricsEmitter *grootfakes.FakeMetricsEmitter
		path           string
	)

	BeforeEach(func() {
		var err error
		path, err = os.MkdirTemp("", "store")
		Expect(err).ToNot(HaveOccurred())
		metricsEmitter = new(grootfakes.FakeMetricsEmitter)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(path)).To(Succeed())
		locksmith.FlockSyscall = syscall.Flock
	})

	Context("ExclusiveLocksmith", func() {
		var exclusiveLocksmith *locksmith.FileSystem

		JustBeforeEach(func() {
			exclusiveLocksmith = locksmith.NewExclusiveFileSystem(path)
		})

		Describe("LockWithTimeout", func() {
			var timeout = 1 * time.Second
			Context("when the lock is not claimed", func() {
				It("claims the lock", func() {
					_, err := exclusiveLocksmith.LockWithTimeout("key", timeout)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the lock is claimed", func() {
				It("returns a timeout error", func() {
					// Initial claim
					_, err := exclusiveLocksmith.LockWithTimeout("key", timeout)
					Expect(err).NotTo(HaveOccurred())

					// Failed 2nd claim
					_, err = exclusiveLocksmith.LockWithTimeout("key", timeout)
					Expect(err).To(MatchError("timed out waiting for the 'key' file lock after '1s'"))
				})
			})

			Context("when the lock is unclaimed within the timeout", func() {
				It("claims the lock", func() {
					lockFd, err := exclusiveLocksmith.LockWithTimeout("key", timeout)
					Expect(err).NotTo(HaveOccurred())

					wentThrough := make(chan struct{})
					go func() {
						defer GinkgoRecover()

						_, err := exclusiveLocksmith.LockWithTimeout("key", timeout)
						Expect(err).NotTo(HaveOccurred())

						close(wentThrough)
					}()

					Consistently(wentThrough).ShouldNot(BeClosed())
					Expect(exclusiveLocksmith.Unlock(lockFd)).To(Succeed())
					Eventually(wentThrough).Should(BeClosed())
				})
			})
		})

		Describe("Lock", func() {
			It("blocks when locking the same key twice", func() {
				lockFd, err := exclusiveLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())

				wentThrough := make(chan struct{})
				go func() {
					defer GinkgoRecover()

					_, err := exclusiveLocksmith.Lock("key")
					Expect(err).NotTo(HaveOccurred())

					close(wentThrough)
				}()

				Consistently(wentThrough).ShouldNot(BeClosed())
				Expect(exclusiveLocksmith.Unlock(lockFd)).To(Succeed())
				Eventually(wentThrough).Should(BeClosed())
			})

			It("creates the lock path when it does not exist", func() {
				Expect(os.RemoveAll(path)).To(Succeed())
				Expect(path).ToNot(BeAnExistingFile())

				_, err := exclusiveLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(BeAnExistingFile())
			})

			It("creates the lock file in the lock path when it does not exist", func() {
				lockFile := filepath.Join(path, "key.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())
				_, err := exclusiveLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			It("removes slashes(/) from key name", func() {
				lockFile := filepath.Join(path, "/tmpkey.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())
				_, err := exclusiveLocksmith.Lock("/tmp/key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			Context("when a metrics emmitter is provided", func() {
				JustBeforeEach(func() {
					exclusiveLocksmith = exclusiveLocksmith.WithMetrics(metricsEmitter)
				})

				It("emits the locking time metric", func() {
					startTime := time.Now()
					_ = filepath.Join(path, "key.lock")
					_, err := exclusiveLocksmith.Lock("key")
					Expect(err).NotTo(HaveOccurred())

					Expect(metricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
					_, metricName, fromTime := metricsEmitter.TryEmitDurationFromArgsForCall(0)

					Expect(metricName).To(Equal(locksmith.ExclusiveMetricsLockingTime))
					Expect(fromTime.Unix()).To(BeNumerically("~", startTime.Unix(), 1))
				})
			})

			Context("when locking the file fails", func() {
				BeforeEach(func() {
					locksmith.FlockSyscall = func(_ int, _ int) error {
						return errors.New("failed to lock file")
					}
				})

				It("returns an error", func() {
					_, err := exclusiveLocksmith.Lock("key")
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
					Expect(exclusiveLocksmith.Unlock(lockFile)).To(
						MatchError(ContainSubstring("failed to unlock file")),
					)
				})
			})
		})
	})

	Context("SharedLocksmith", func() {
		var sharedLocksmith *locksmith.FileSystem
		JustBeforeEach(func() {
			sharedLocksmith = locksmith.NewSharedFileSystem(path)
		})

		Describe("LockWithTimeout", func() {
			var timeout = 1 * time.Second
			Context("when the lock is not claimed", func() {
				It("claims the lock", func() {
					_, err := sharedLocksmith.LockWithTimeout("key", timeout)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the lock is claimed", func() {
				It("doesn't return an error because this is a shared lock", func() {
					// This one claims the lock successfully
					_, err := sharedLocksmith.LockWithTimeout("key", timeout)
					Expect(err).NotTo(HaveOccurred())

					// This one tries to claim the inuse lock
					_, err = sharedLocksmith.LockWithTimeout("key", timeout)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Describe("Lock", func() {
			It("does not block when locking the same key twice", func() {
				lockFd1, err := sharedLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())

				lockFd2, err := sharedLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())

				err = sharedLocksmith.Unlock(lockFd1)
				Expect(err).NotTo(HaveOccurred())

				err = sharedLocksmith.Unlock(lockFd2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates the lock file in the lock path when it does not exist", func() {
				lockFile := filepath.Join(path, "key.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())

				_, err := sharedLocksmith.Lock("key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			It("removes slashes(/) from key name", func() {
				lockFile := filepath.Join(path, "/tmpkey.lock")

				Expect(lockFile).ToNot(BeAnExistingFile())
				_, err := sharedLocksmith.Lock("/tmp/key")
				Expect(err).NotTo(HaveOccurred())
				Expect(lockFile).To(BeAnExistingFile())
			})

			Context("when a metrics emmitter is provided", func() {
				JustBeforeEach(func() {
					sharedLocksmith = sharedLocksmith.WithMetrics(metricsEmitter)
				})

				It("emits the locking time metric", func() {
					startTime := time.Now()
					_ = filepath.Join(path, "key.lock")
					_, err := sharedLocksmith.Lock("key")
					Expect(err).NotTo(HaveOccurred())

					Expect(metricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
					_, metricName, fromTime := metricsEmitter.TryEmitDurationFromArgsForCall(0)

					Expect(metricName).To(Equal(locksmith.SharedMetricsLockingTime))
					Expect(fromTime.Unix()).To(BeNumerically("~", startTime.Unix(), 1))
				})
			})

			Context("when locking the file fails", func() {
				BeforeEach(func() {
					locksmith.FlockSyscall = func(_ int, _ int) error {
						return errors.New("failed to lock file")
					}
				})

				It("returns an error", func() {
					_, err := sharedLocksmith.Lock("key")
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
					Expect(sharedLocksmith.Unlock(lockFile)).To(
						MatchError(ContainSubstring("failed to unlock file")),
					)
				})
			})
		})
	})
})
