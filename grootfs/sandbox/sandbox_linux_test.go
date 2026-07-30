package sandbox_test

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/sandbox"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func init() {
	sandbox.Register("test-action", func(logger lager.Logger, extraFiles []*os.File, args ...string) error {
		cmd := os.Args[1]

		switch cmd {
		case "say-hello":
			fmt.Print("Hi from action!")
		case "pwd":
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			fmt.Print(dir)
		case "get-userns":
			ns, err := getCurrentUserNamespace()
			if err != nil {
				return err
			}
			fmt.Print(ns)
		case "echo-stdin":
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			fmt.Print(string(content))
		case "cat-extra-file":
			content, err := io.ReadAll(extraFiles[0])
			if err != nil {
				return err
			}
			fmt.Print(string(content))
		case "log":
			logger.Info(args[1])
		}

		return nil
	})

	if reexec.Init() {
		// prevents infinite reexec loop
		// Details: https://medium.com/@teddyking/namespaces-in-go-reexec-3d1295b91af8
		os.Exit(0)
	}
}

var _ = Describe("Sandbox Rexecer", func() {
	var (
		logger        *lagertest.TestLogger
		idMapper      sandbox.IDMapper
		commandRunner commandrunner.CommandRunner
		idMappings    groot.IDMappings

		reexecer groot.SandboxReexecer
	)

	BeforeEach(func() {
		mappings := []groot.IDMappingSpec{
			{HostID: 5000, NamespaceID: 0, Size: 1},
			{HostID: 100000, NamespaceID: 1, Size: 65000},
		}
		idMappings = groot.IDMappings{
			UIDMappings: mappings,
			GIDMappings: mappings,
		}

		logger = lagertest.NewTestLogger("test")
		commandRunner = linux_command_runner.New()
		idMapper = unpackerpkg.NewIDMapper("newuidmap", "newgidmap", commandRunner)

	})

	JustBeforeEach(func() {
		reexecer = sandbox.NewReexecer(logger, idMapper, idMappings)
	})

	It("executes the action", func() {
		out, err := reexecer.Reexec("test-action", groot.ReexecSpec{Args: []string{"say-hello"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(Equal("Hi from action!"))
	})

	It("preserves the current root", func() {
		out, err := reexecer.Reexec("test-action", groot.ReexecSpec{Args: []string{"pwd"}})
		Expect(err).NotTo(HaveOccurred())
		currentDir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(Equal(currentDir))
	})

	It("runs in the same namespace", func() {
		currentUserNs, err := getCurrentUserNamespace()
		Expect(err).NotTo(HaveOccurred())
		out, err := reexecer.Reexec("test-action", groot.ReexecSpec{Args: []string{"get-userns"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(Equal(currentUserNs))
	})

	It("propagates stdin", func() {
		buf := bytes.NewBufferString("some-stuff")
		out, err := reexecer.Reexec("test-action", groot.ReexecSpec{
			Args:  []string{"echo-stdin"},
			Stdin: buf,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(Equal("some-stuff"))
	})

	It("relogs", func() {
		_, err := reexecer.Reexec("test-action", groot.ReexecSpec{
			Args: []string{"log", "some-log"},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(logger.LogMessages()).To(ContainElement(ContainSubstring("some-log")))
	})

	Context("when passed an extra file ", func() {
		var file *os.File

		BeforeEach(func() {
			var err error
			file, err = os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			_, err = file.WriteString("some-stuff")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			file.Close()
			Expect(os.Remove(file.Name())).To(Succeed())
		})

		It("receives the extra file", func() {
			out, err := reexecer.Reexec("test-action", groot.ReexecSpec{
				Args:       []string{"cat-extra-file"},
				ExtraFiles: []string{file.Name()},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).To(Equal("some-stuff"))
		})
	})

	Context("when passed an extra file that does not exist", func() {
		It("fails", func() {
			_, err := reexecer.Reexec("test-action", groot.ReexecSpec{ExtraFiles: []string{"/foo/bar"}})
			Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
		})
	})

	Context("when asked to chroot", func() {
		var chrootDir string

		BeforeEach(func() {
			var err error
			chrootDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(chrootDir)).To(Succeed())
		})

		It("chroots", func() {
			out, err := reexecer.Reexec("test-action", groot.ReexecSpec{
				Args:      []string{"pwd"},
				ChrootDir: chrootDir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).To(Equal("/"))
		})

		Context("when passed an extra file outside the chroot dir", func() {
			var file *os.File

			BeforeEach(func() {
				var err error
				file, err = os.CreateTemp("", "")
				Expect(err).NotTo(HaveOccurred())
				_, err = file.WriteString("some-stuff")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				file.Close()
				Expect(os.Remove(file.Name())).To(Succeed())
			})

			It("receives the extra file", func() {
				out, err := reexecer.Reexec("test-action", groot.ReexecSpec{
					Args:       []string{"cat-extra-file"},
					ExtraFiles: []string{file.Name()},
					ChrootDir:  chrootDir,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).To(Equal("some-stuff"))
			})
		})
	})

	Context("when asked to clone userns", func() {
		BeforeEach(func() {
			mappings := []groot.IDMappingSpec{
				{HostID: 0, NamespaceID: 0, Size: 1},
			}
			idMappings = groot.IDMappings{
				UIDMappings: mappings,
				GIDMappings: mappings,
			}
		})

		It("clones the user namespace", func() {
			currentUserNs, err := getCurrentUserNamespace()
			Expect(err).NotTo(HaveOccurred())
			out, err := reexecer.Reexec("test-action", groot.ReexecSpec{CloneUserns: true, Args: []string{"get-userns"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).NotTo(Equal(currentUserNs))
		})
	})

	Context("when the command being reexeced is not registered", func() {
		It("returns an error", func() {
			_, err := reexecer.Reexec("random-command", groot.ReexecSpec{})
			Expect(err).To(MatchError("unregistered command: random-command"))
		})
	})

})

func getCurrentUserNamespace() (string, error) {
	return os.Readlink("/proc/self/ns/user")
}
