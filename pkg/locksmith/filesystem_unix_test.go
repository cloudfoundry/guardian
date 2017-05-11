// +build !windows

package locksmith_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/guardian/pkg/locksmith"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filesystem", func() {
	var (
		fileSystemLock *locksmith.FileSystem
		lockDirPath    string
		lockPath       string
	)

	BeforeEach(func() {
		var err error
		lockDirPath, err = ioutil.TempDir("", "")
		Expect(err).ToNot(HaveOccurred())

		fileSystemLock = locksmith.NewFileSystem()

		lockPath = filepath.Join(lockDirPath, "a_lock.lock")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(lockDirPath)).To(Succeed())
	})

	Describe("Lock", func() {
		It("creates the lock file when it does not exist", func() {
			Expect(lockPath).ToNot(BeAnExistingFile())
			_, err := fileSystemLock.Lock(lockPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(lockPath).To(BeAnExistingFile())
		})

		Context("when the parent path does not exist", func() {
			It("returns an error", func() {
				_, err := fileSystemLock.Lock("/path/to/non/existent/lock")
				Expect(err).To(MatchError(ContainSubstring("creating lock file for path")))
			})
		})

		Context("when locking the file fails", func() {
			BeforeEach(func() {
				locksmith.FlockSyscall = func(_ int, _ int) error {
					return errors.New("failed to lock file")
				}
			})

			AfterEach(func() {
				locksmith.FlockSyscall = syscall.Flock
			})

			It("returns an error", func() {
				_, err := fileSystemLock.Lock(lockPath)
				Expect(err).To(MatchError(ContainSubstring("failed to lock file")))
			})
		})
	})

	Describe("Unlocker", func() {
		var unlocker locksmith.Unlocker

		Describe("Unlock", func() {
			Context("when the lock is held", func() {
				BeforeEach(func() {
					var err error
					unlocker, err = fileSystemLock.Lock(lockPath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("releases the lock", func() {
					wentThrough := make(chan struct{})
					go func() {
						defer GinkgoRecover()

						_, err := fileSystemLock.Lock(lockPath)
						Expect(err).NotTo(HaveOccurred())

						close(wentThrough)
					}()

					Consistently(wentThrough).ShouldNot(BeClosed())
					Expect(unlocker.Unlock()).To(Succeed())
					Eventually(wentThrough).Should(BeClosed())
				})

				Context("when unlocking a file descriptor fails", func() {
					BeforeEach(func() {
						locksmith.FlockSyscall = func(_ int, _ int) error {
							return errors.New("failed to unlock file")
						}
					})

					AfterEach(func() {
						locksmith.FlockSyscall = syscall.Flock
					})

					It("returns an error", func() {
						Expect(unlocker.Unlock()).To(
							MatchError(ContainSubstring("failed to unlock file")))
					})
				})
			})
		})
	})
})
