package stopper_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/cloudfoundry-incubator/guardian/rundmc/stopper"
	"github.com/cloudfoundry-incubator/guardian/rundmc/stopper/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("CgroupStopper", func() {
	var (
		fakeCgroupResolver *fakes.FakeCgroupPathResolver
		fakeKiller         *fakes.FakeKiller

		subject *stopper.CgroupStopper
	)

	BeforeEach(func() {
		fakeCgroupResolver = new(fakes.FakeCgroupPathResolver)
		fakeKiller = new(fakes.FakeKiller)

		fakeCgroupDir, err := ioutil.TempDir("", "fakecgroupdir")
		Expect(err).NotTo(HaveOccurred())

		devicesCgroupPath := filepath.Join(fakeCgroupDir, "foo", "devices")
		Expect(os.MkdirAll(devicesCgroupPath, 0700)).To(Succeed())
		Expect(ioutil.WriteFile(filepath.Join(devicesCgroupPath, "cgroup.procs"), []byte(`1
3
5
9`), 0700)).To(Succeed())

		fakeCgroupResolver.ResolveStub = func(name string, subsystem string) (string, error) {
			return filepath.Join(fakeCgroupDir, name, subsystem), nil
		}

		subject = stopper.New(fakeCgroupResolver, fakeKiller)
	})

	It("sends TERM to all the processes found in the cgroup", func() {
		Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", nil, true)).To(Succeed())

		sig, pids := fakeKiller.KillArgsForCall(0)
		Expect(sig).To(Equal(syscall.SIGTERM))
		Expect(pids).To(ConsistOf(1, 3, 5, 9))
	})

	It("does not send TERM to processes in the exceptions list", func() {
		Expect(subject.StopAll(lagertest.NewTestLogger("test"), "foo", []int{
			3, 5,
		}, false)).To(Succeed())

		Expect(fakeKiller.KillCallCount()).To(Equal(1))

		sig, pids := fakeKiller.KillArgsForCall(0)
		Expect(sig).To(Equal(syscall.SIGTERM))
		Expect(pids).To(ConsistOf(1, 9))
	})
})
