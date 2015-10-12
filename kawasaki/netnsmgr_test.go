package kawasaki_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NetnsMgr", func() {
	var (
		fakeRunner *fake_command_runner.FakeCommandRunner
		netnsDir   string
		mgr        kawasaki.NetnsMgr
		logger     lager.Logger
	)

	BeforeEach(func() {
		netnsDir = tmpDir()
		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		mgr = kawasaki.NewManager(fakeRunner, netnsDir)
	})

	AfterEach(func() {
		os.RemoveAll(netnsDir)
	})

	Describe("Creating a Network Namespace", func() {
		It("creates a namespace using 'ip netns add'", func() {
			Expect(mgr.Create(logger, "my-namespace")).To(Succeed())

			Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "ip",
				Args: []string{
					"netns", "add", "my-namespace",
				},
			}))
		})

		Context("when the command fails", func() {
			It("returns an error", func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{},
					func(*exec.Cmd) error {
						return errors.New("banana")
					},
				)

				Expect(mgr.Create(logger, "my-namespace")).NotTo(Succeed())
			})
		})
	})

	Describe("Looking up a Network Namespace path", func() {
		It("looks up the Network Namespace path", func() {
			Expect(ioutil.WriteFile(path.Join(netnsDir, "banana"), []byte(""), 0700)).To(Succeed())

			path, theUnexpected := mgr.Lookup(logger, "banana")
			Expect(theUnexpected).NotTo(HaveOccurred())
			Expect(path).To(Equal(filepath.Join(netnsDir, "banana")))
		})

		Context("when the namespace does not exist", func() {
			It("returns an error", func() {
				_, theUnexpected := mgr.Lookup(logger, "banana")
				Expect(theUnexpected).To(HaveOccurred())
			})
		})
	})

	Describe("Destroying a Network Namespace", func() {
		It("destroys the network using 'ip netns delete'", func() {
			Expect(mgr.Destroy(logger, "my-namespace")).To(Succeed())

			Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "ip",
				Args: []string{
					"netns", "delete", "my-namespace",
				},
			}))
		})

		Context("when the command fails", func() {
			It("returns an error", func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{},
					func(*exec.Cmd) error {
						return errors.New("banana")
					},
				)

				Expect(mgr.Destroy(logger, "my-namespace")).NotTo(Succeed())
			})
		})
	})
})

func tmpDir() string {
	f, e := ioutil.TempDir("", "")
	Expect(e).NotTo(HaveOccurred())
	return f
}
