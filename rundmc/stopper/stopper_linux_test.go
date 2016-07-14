package stopper_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/guardian/rundmc/stopper"
	fakes "code.cloudfoundry.org/guardian/rundmc/stopper/stopperfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CgroupStopper", func() {
	var (
		fakeCgroupResolver *fakes.FakeCgroupPathResolver
		fakeKiller         *fakes.FakeKiller
		fakeRetrier        *fakes.FakeRetrier

		subject                          *stopper.CgroupStopper
		devicesCgroupPath, fakeCgroupDir string
	)

	BeforeEach(func() {
		fakeCgroupResolver = new(fakes.FakeCgroupPathResolver)
		fakeKiller = new(fakes.FakeKiller)
		fakeRetrier = new(fakes.FakeRetrier)

		var err error
		fakeCgroupDir, err = ioutil.TempDir("", "fakecgroupdir")
		Expect(err).NotTo(HaveOccurred())

		devicesCgroupPath = filepath.Join(fakeCgroupDir, "foo", "devices")
		Expect(os.MkdirAll(devicesCgroupPath, 0700)).To(Succeed())
		Expect(ioutil.WriteFile(filepath.Join(devicesCgroupPath, "cgroup.procs"), []byte(`1
3
5
9`), 0700)).To(Succeed())

		fakeCgroupResolver.ResolveStub = func(name string, subsystem string) (string, error) {
			return filepath.Join(fakeCgroupDir, name, subsystem), nil
		}

		fakeRetrier.RunStub = func(fn func() error) error {
			return fn()
		}

		subject = stopper.New(fakeCgroupResolver, fakeKiller, fakeRetrier)
	})

	AfterEach(func() {
		os.RemoveAll(fakeCgroupDir)
	})

	It("does not send any signal to processes in the exceptions list", func() {
		Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{3, 5}, false)).To(Succeed())
		Expect(fakeKiller).To(HaveKilled(0, syscall.SIGTERM, 1, 9))
		Expect(fakeKiller).To(HaveKilled(1, syscall.SIGKILL, 1, 9))
	})

	Context("when the kill flag is true", func() {
		It("sends a KILL to all processes found in the cgroup", func() {
			Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", nil, true)).To(Succeed())
			Expect(fakeKiller).To(HaveKilled(0, syscall.SIGKILL, 1, 3, 5, 9))
		})

		It("does not send TERM to processes found in the cgroup", func() {
			Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", nil, true)).To(Succeed())
			Expect(fakeKiller).NotTo(HaveKilled(0, syscall.SIGTERM, 1, 3, 5, 9))
		})
	})

	Context("when the kill flag is false", func() {
		It("eventually returns successfully even if the cgroup.procs is unchanged (because it eventually gives up and SIGKILLs)", func() {
			Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{3, 5}, false)).To(Succeed())
		})

		It("sends TERM to all the processes found in the cgroup", func() {
			Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", nil, false)).To(Succeed())
			Expect(fakeKiller).To(HaveKilled(0, syscall.SIGTERM, 1, 3, 5, 9))
		})

		It("repeatedly sends TERM and KILL to the remaining processes until the retrier gives up", func() {
			fakeRetrier.RunStub = func(fn func() error) error {
				Expect(ioutil.WriteFile(filepath.Join(devicesCgroupPath, "cgroup.procs"), []byte(`3
9`), 0700)).To(Succeed())
				err := fn()
				return err
			}

			Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{9}, false)).To(Succeed())

			Expect(fakeKiller).To(HaveKilled(0, syscall.SIGTERM, 3))
			Expect(fakeKiller).To(HaveKilled(1, syscall.SIGKILL, 3))
			Expect(fakeRetrier.RunCallCount()).To(Equal(2))
		})

		Describe("telling the retrier whether it should continue", func() {
			It("tells the retrier that it is done when there are no processes left", func() {
				fakeRetrier.RunStub = func(fn func() error) error {
					Expect(ioutil.WriteFile(filepath.Join(devicesCgroupPath, "cgroup.procs"), []byte(`9
`), 0700)).To(Succeed())
					Expect(fn()).To(Succeed()) // should stop retrying when everything's gone
					return nil
				}

				Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{9}, false)).To(Succeed())

				Expect(fakeRetrier.RunCallCount()).To(Equal(2))
			})

			It("tells the retrier that it is not yet done if there are processes left", func() {
				fakeRetrier.RunStub = func(fn func() error) error {
					Expect(fn()).NotTo(Succeed()) // should not stop retrying until everything's gone
					return nil
				}

				Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{9}, false)).To(Succeed())

				Expect(fakeKiller).To(HaveKilled(0, syscall.SIGTERM, 1, 3, 5))
				Expect(fakeKiller).To(HaveKilled(1, syscall.SIGKILL, 1, 3, 5))
				Expect(fakeRetrier.RunCallCount()).To(Equal(2))
			})
		})

		Context("and processes are still in cgroup.procs after the TERM was sent as often as the retrier is willing", func() {
			It("eventually sends KILL to processes", func() {
				Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{3, 5}, false)).To(Succeed())
				Expect(fakeKiller).To(HaveKilled(1, syscall.SIGKILL, 1, 9))
			})

			It("always returns success if it killed, because kill always works", func() {
				Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{3, 5}, false)).To(Succeed())
			})
		})
	})
})

type haveKilledMatcher struct {
	call int
	sig  syscall.Signal
	pids []int

	message string
}

func HaveKilled(call int, signal syscall.Signal, pids ...int) *haveKilledMatcher {
	return &haveKilledMatcher{
		call: call,
		sig:  signal,
		pids: pids,
	}
}

func (h *haveKilledMatcher) Match(actual interface{}) (success bool, err error) {
	fakeKiller := actual.(*fakes.FakeKiller)
	if fakeKiller.KillCallCount() <= h.call {
		h.message = fmt.Sprintf("to have been called at least %d times, was called %d times", h.call+1, fakeKiller.KillCallCount())
		return false, nil
	}

	s, p := fakeKiller.KillArgsForCall(h.call)

	matchSig, err := Equal(h.sig).Match(s)
	if err != nil {
		return false, err
	}

	matchPid, err := ConsistOf(h.pids).Match(p)
	if err != nil {
		return false, err
	}

	if !matchSig {
		h.message = fmt.Sprintf("have used signal %s, instead used %s", h.sig, s)
	}

	if !matchPid {
		h.message = fmt.Sprintf("have signalled pids %v, instead signalled %v", h.pids, p)
	}

	return matchSig && matchPid, nil
}

func (h *haveKilledMatcher) FailureMessage(actual interface{}) string {
	return "Expected killer to " + h.message
}

func (h *haveKilledMatcher) NegatedFailureMessage(actual interface{}) string {
	return "Expected killer not to " + h.message
}
