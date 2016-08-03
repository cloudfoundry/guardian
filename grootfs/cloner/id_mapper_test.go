package cloner_test

import (
	"os/exec"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/groot"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IDMapper", func() {
	var (
		cmdPath       string
		fakeCmdRunner *fake_command_runner.FakeCommandRunner
		idMapper      *cloner.CommandIDMapper
	)

	JustBeforeEach(func() {
		fakeCmdRunner = fake_command_runner.New()
		fakeCmdRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: cmdPath,
		}, func(cmd *exec.Cmd) error {
			// Avoid calling the OS command
			return nil
		})
	})

	Describe("MapUIDs", func() {
		BeforeEach(func() {
			cmdPath = "newuidmap"
		})

		It("uses the newuidmap correctly", func() {
			idMapper = cloner.NewIDMapper(fakeCmdRunner)

			Expect(idMapper.MapUIDs(1000, []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 30},
				groot.IDMappingSpec{HostID: 100, NamespaceID: 200, Size: 300},
			})).To(Succeed())

			cmds := fakeCmdRunner.ExecutedCommands()
			newuidCmd := cmds[0]

			Expect(newuidCmd.Args[0]).To(Equal("newuidmap"))
			Expect(newuidCmd.Args[1]).To(Equal("1000"))
			Expect(newuidCmd.Args[2:]).To(Equal([]string{"20", "10", "30", "200", "100", "300"}))
		})
	})

	Describe("MapGIDs", func() {
		BeforeEach(func() {
			cmdPath = "newgidmap"
		})

		It("uses the newgidmap correctly", func() {
			idMapper = cloner.NewIDMapper(fakeCmdRunner)

			Expect(idMapper.MapGIDs(1000, []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: 50, NamespaceID: 60, Size: 70},
				groot.IDMappingSpec{HostID: 400, NamespaceID: 500, Size: 600},
			})).To(Succeed())

			cmds := fakeCmdRunner.ExecutedCommands()
			newgidCmd := cmds[0]

			Expect(newgidCmd.Args[0]).To(Equal("newgidmap"))
			Expect(newgidCmd.Args[1]).To(Equal("1000"))
			Expect(newgidCmd.Args[2:]).To(Equal([]string{"60", "50", "70", "500", "400", "600"}))
		})
	})

})
