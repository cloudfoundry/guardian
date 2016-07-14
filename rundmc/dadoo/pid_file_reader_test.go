package dadoo_test

import (
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/guardian/rundmc/dadoo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
)

var _ = Describe("PidFileReader", func() {

	var (
		clk     *fakeclock.FakeClock
		timeout time.Duration

		pdr *dadoo.PidFileReader

		pidFileContents string
		pidFilePath     string
	)

	BeforeEach(func() {
		clk = fakeclock.NewFakeClock(time.Now())
		timeout = time.Millisecond * 60

		pidFileContents = "5621"
	})

	JustBeforeEach(func() {
		pdr = &dadoo.PidFileReader{
			Clock:         clk,
			Timeout:       timeout,
			SleepInterval: 20 * time.Millisecond,
		}

		pidFile, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		_, err = pidFile.Write([]byte(pidFileContents))
		Expect(err).NotTo(HaveOccurred())

		pidFilePath = pidFile.Name()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(pidFilePath)).To(Succeed())
	})

	It("should read the pid file", func() {
		pid, err := pdr.Pid(pidFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(pid).To(Equal(5621))
	})

	Context("when pid file does not exist", func() {
		JustBeforeEach(func() {
			Expect(os.RemoveAll(pidFilePath)).To(Succeed())
		})

		Context("and it is eventually created", func() {
			It("should read the pid file", func(done Done) {
				pidReturns := make(chan struct{})
				go func() {
					defer GinkgoRecover()

					pid, err := pdr.Pid(pidFilePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(pid).To(Equal(5621))

					close(pidReturns)
				}()

				// WaitForWatchersAndIncrement ensures that the implementation will try
				// first time and sleep. However the sleep interval is 20ms so
				// incrementing by 10ms won't get the loop moving. We do that to ensure
				// that the file write will happen before the implementation tries to
				// read the file. Hence, after we write the file the clock is
				// incremented by a further 10ms.
				clk.WaitForWatcherAndIncrement(time.Millisecond * 10)
				Expect(ioutil.WriteFile(pidFilePath, []byte("5621"), 0766)).To(Succeed())
				clk.Increment(time.Millisecond * 10)

				Eventually(pidReturns).Should(BeClosed())
				close(done)
			}, 1.0)
		})

		Context("and it is never created", func() {
			It("should return error after the timeout", func(done Done) {
				pidReturns := make(chan struct{})
				go func() {
					defer GinkgoRecover()

					_, err := pdr.Pid(pidFilePath)
					Expect(err).To(MatchError(ContainSubstring("timeout")))

					close(pidReturns)
				}()

				for i := 0; i < 3; i++ {
					clk.WaitForWatcherAndIncrement(time.Millisecond * 20)
				} // 3 * 20ms = 60ms

				Eventually(pidReturns).Should(BeClosed())
				close(done)
			}, 1.0)
		})
	})

	Context("when the pid file is empty", func() {
		JustBeforeEach(func() {
			f, err := os.OpenFile(pidFilePath, os.O_TRUNC, 0766)
			Expect(err).NotTo(HaveOccurred())
			Expect(f.Close()).To(Succeed())
		})

		Context("and it is eventually populated", func() {
			It("should read the pid file", func(done Done) {
				pidReturns := make(chan struct{})
				go func() {
					defer GinkgoRecover()

					pid, err := pdr.Pid(pidFilePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(pid).To(Equal(5621))

					close(pidReturns)
				}()

				// WaitForWatchersAndIncrement ensures that the implementation will try
				// first time and sleep. However the sleep interval is 20ms so
				// incrementing by 10ms won't get the loop moving. We do that to ensure
				// that the file write will happen before the implementation tries to
				// read the file. Hence, after we write the file the clock is
				// incremented by a further 10ms.
				clk.WaitForWatcherAndIncrement(time.Millisecond * 10)
				Expect(ioutil.WriteFile(pidFilePath, []byte("5621"), 0766)).To(Succeed())
				clk.Increment(time.Millisecond * 10)

				Eventually(pidReturns).Should(BeClosed())
				close(done)
			}, 1.0)
		})

		Context("and it is never populated", func() {
			It("should return error after the timeout", func(done Done) {
				pidReturns := make(chan struct{})
				go func() {
					defer GinkgoRecover()

					_, err := pdr.Pid(pidFilePath)
					Expect(err).To(MatchError(ContainSubstring("timeout")))

					close(pidReturns)
				}()

				for i := 0; i < 3; i++ {
					clk.WaitForWatcherAndIncrement(time.Millisecond * 20)
				} // 3 * 20ms = 60ms

				Eventually(pidReturns).Should(BeClosed())
				close(done)
			}, 1.0)
		})
	})

	Context("when pid file does not contain an int value", func() {
		BeforeEach(func() {
			pidFileContents = "notanint"
		})

		It("should return error", func() {
			_, err := pdr.Pid(pidFilePath)
			Expect(err).To(MatchError(ContainSubstring("parsing pid file contents")))
		})
	})
})
