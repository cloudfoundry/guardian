package rundmc_test

import (
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/cloudfoundry-incubator/garden-shed/pkg/retrier"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("StateChecker", func() {
	var (
		checker *rundmc.StateChecker
		logger  lager.Logger
		tmp     string
		clk     clock.Clock
	)

	BeforeEach(func() {
		clk = clock.NewClock()
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		var err error
		tmp, err = ioutil.TempDir("", "startchecktest")
		Expect(err).NotTo(HaveOccurred())

		retrier := retrier.Retrier{
			Timeout:         100 * time.Millisecond,
			PollingInterval: 10 * time.Millisecond,
			Clock:           clk,
		}

		checker = &rundmc.StateChecker{
			StateFileDir: tmp,
			Retrier:      retrier,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmp)).To(Succeed())
	})

	Describe("State", func() {
		It("returns the state of the container", func() {
			Expect(os.MkdirAll(path.Join(tmp, "some-id"), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(tmp, "some-id", "state.json"), []byte(`{"init_process_pid":42}`), 0700)).To(Succeed())

			state, err := checker.State(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())

			Expect(state.Pid).To(Equal(42))
		})

		Context("when the state file does not contain valid JSON", func() {
			It("should return an error", func() {
				Expect(os.MkdirAll(path.Join(tmp, "some-id"), 0700)).To(Succeed())
				Expect(ioutil.WriteFile(path.Join(tmp, "some-id", "state.json"), []byte(`"init_process_pid":"4`), 0700)).To(Succeed())

				_, err := checker.State(logger, "some-id")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the state file does not exist", func() {

			It("returns an error", func() {
				_, err := checker.State(logger, "some-id")
				Expect(err.Error()).To(ContainSubstring("state.json: no such file or directory"))
			})

			Context("and later on appears", func() {
				var (
					fakeClock *fakeclock.FakeClock
				)

				BeforeEach(func() {
					fakeClock = fakeclock.NewFakeClock(time.Now())
					clk = fakeClock
				})

				JustBeforeEach(func() {
					go func() {
						for i := 0; i < 10; i++ {
							if i == 5 {
								// write the state.json some time later
								Expect(os.MkdirAll(path.Join(tmp, "some-id"), 0700)).To(Succeed())
								Expect(ioutil.WriteFile(path.Join(tmp, "some-id", "state.json"), []byte(`{"init_process_pid":42}`), 0700)).To(Succeed())
							}
							fakeClock.WaitForWatcherAndIncrement(10 * time.Millisecond)
						}
					}()
				})

				It("returns the state of the container", func() {
					state, err := checker.State(logger, "some-id")
					Expect(err).NotTo(HaveOccurred())
					Expect(state.Pid).To(Equal(42))
				})
			})
		})
	})
})
