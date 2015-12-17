package rundmc_test

import (
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("StateChecker", func() {
	var (
		checker *rundmc.StateChecker
		logger  lager.Logger
		tmp     string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		var err error
		tmp, err = ioutil.TempDir("", "startchecktest")
		Expect(err).NotTo(HaveOccurred())

		checker = &rundmc.StateChecker{
			StateFileDir: tmp,
			Timeout:      800 * time.Millisecond,
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
	})
})
