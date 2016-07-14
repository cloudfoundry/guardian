package bundlerules_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("RootFS", func() {
	var (
		rule          bundlerules.RootFS
		commandRunner *fake_command_runner.FakeCommandRunner

		rootfsPath     string
		returnedBundle goci.Bndl
		privileged     bool
	)

	BeforeEach(func() {
		privileged = false
	})

	JustBeforeEach(func() {
		rootfsPath = "banana/"
		commandRunner = fake_command_runner.New()

		rule = bundlerules.RootFS{
			ContainerRootUID: 999,
			ContainerRootGID: 888,

			MkdirChown: bundlerules.ChrootMkdir{
				Command: func(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd {
					return exec.Command("reexeced-thing", append(
						[]string{
							"-rootfsPath", rootfsPath,
							"-uid", strconv.Itoa(uid),
							"-gid", strconv.Itoa(gid),
							"-recreate", fmt.Sprintf("%t", recreate),
							"-perm", strconv.FormatUint(uint64(mode.Perm()), 8),
						}, paths...)...)
				},

				CommandRunner: commandRunner,
			},
		}

		returnedBundle = rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			RootFSPath: rootfsPath,
			Privileged: privileged,
		})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootfsPath)).To(Succeed())
	})

	It("applies the rootfs to the passed bundle", func() {
		Expect(returnedBundle.Spec.Root.Path).To(Equal(rootfsPath))
	})

	Describe("creating needed directories", func() {
		Context("when the container is privileged", func() {
			BeforeEach(func() {
				privileged = true
			})

			It("pre-creates needed directories with the correct ownership", func() {
				Expect(commandRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "reexeced-thing",
						Args: []string{
							"-rootfsPath", rootfsPath,
							"-uid", "0",
							"-gid", "0",
							"-recreate", "true",
							"-perm", "755",
							// this is a workaround for our current aufs code not properly changing the
							// ownership of / to container-root. without this step runc is unable to
							// pivot root in user-namespaced containers.
							".pivot_root",
							// stuff in this directory frequently confuses runc, and poses a potential
							// security vulnerability.
							"dev",
							// we ask runc to mount both of these, so we need to ensure they exist
							"proc",
							"sys",
						},
					}))
			})

			It("pre-creates /tmp as ugo+rwx if it doesn't exist", func() {
				Expect(commandRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "reexeced-thing",
						Args: []string{
							"-rootfsPath", rootfsPath,
							"-uid", "0",
							"-gid", "0",
							"-recreate", "false",
							"-perm", "777",
							// this is a workaround for our current aufs code not properly changing the
							// ownership of / to container-root. without this step runc is unable to
							// pivot root in user-namespaced containers.
							"tmp",
						},
					}))
			})
		})

		Context("when the container is UNprivileged", func() {
			BeforeEach(func() {
				privileged = false
			})

			It("pre-creates needed directories with the correct ownership", func() {
				Expect(commandRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "reexeced-thing",
						Args: []string{
							"-rootfsPath", rootfsPath,
							"-uid", "999",
							"-gid", "888",
							"-recreate", "true",
							"-perm", "755",
							// this is a workaround for our current aufs code not properly changing the
							// ownership of / to container-root. without this step runc is unable to
							// pivot root in user-namespaced containers.
							".pivot_root",
							// stuff in this directory frequently confuses runc, and poses a potential
							// security vulnerability.
							"dev",
							// we ask runc to mount both of these, so we need to ensure they exist
							"proc",
							"sys",
						},
					}))
			})

			It("pre-creates /tmp as ugo+rwx if it doesn't exist", func() {
				Expect(commandRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "reexeced-thing",
						Args: []string{
							"-rootfsPath", rootfsPath,
							"-uid", "999",
							"-gid", "888",
							"-recreate", "false",
							"-perm", "777",
							// this is a workaround for our current aufs code not properly changing the
							// ownership of / to container-root. without this step runc is unable to
							// pivot root in user-namespaced containers.
							"tmp",
						},
					}))
			})
		})
	})
})
