package cloner_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TarCloner", func() {
	var (
		logger lager.Logger

		fromDir string
		toDir   string

		fakeCmdRunner *fake_command_runner.FakeCommandRunner
		fakeIDMapper  *clonerfakes.FakeIDMapper
		tarCloner     *cloner.TarCloner
		realTar       *exec.Cmd
		err           error
	)

	BeforeEach(func() {
		fromDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(fromDir, "a_file"), []byte("hello-world"), 0600)).To(Succeed())

		tempDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		toDir = path.Join(tempDir, "rootfs")

		fakeCmdRunner = fake_command_runner.New()
		fakeIDMapper = new(clonerfakes.FakeIDMapper)

		fakeCmdRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: os.Args[0],
		}, func(cmd *exec.Cmd) error {
			realTar = exec.Command("tar", cmd.Args[3:]...)
			realTar.Stdin = cmd.Stdin
			realTar.Stdout = cmd.Stdout
			realTar.Stderr = cmd.Stderr
			err := realTar.Start()
			cmd.Process = realTar.Process
			return err
		})
		fakeCmdRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
			Path: os.Args[0],
		}, func(cmd *exec.Cmd) error {
			return realTar.Wait()
		})
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-graph")
		tarCloner = cloner.NewTarCloner(fakeCmdRunner, fakeIDMapper)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(fromDir)).To(Succeed())
		Expect(os.RemoveAll(toDir)).To(Succeed())
	})

	It("should have the image contents in the rootfs directory", func() {
		Expect(tarCloner.Clone(logger, groot.CloneSpec{
			FromDir: fromDir,
			ToDir:   toDir,
		})).To(Succeed())

		filePath := path.Join(toDir, "a_file")
		Expect(filePath).To(BeARegularFile())
		contents, err := ioutil.ReadFile(filePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("hello-world"))
	})

	Context("when using mapping users", func() {
		Describe("UIDMappings", func() {
			It("should use the uid provided", func() {
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
					UIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
					},
				})).To(Succeed())

				Expect(fakeIDMapper.MapUIDsCallCount()).To(Equal(1))
				pid, mappings := fakeIDMapper.MapUIDsArgsForCall(0)

				Expect(pid).To(Equal(realTar.Process.Pid))
				Expect(mappings).To(Equal([]groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
				}))
			})

			Context("when it fails", func() {
				BeforeEach(func() {
					fakeIDMapper.MapUIDsReturns(errors.New("Boom!"))
				})

				It("should return an error", func() {
					Expect(tarCloner.Clone(logger, groot.CloneSpec{
						FromDir: fromDir,
						ToDir:   toDir,
						UIDMappings: []groot.IDMappingSpec{
							groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
						},
					})).To(MatchError("uid mapping: Boom!"))
				})
			})
		})

		Describe("GIDMappings", func() {
			It("should use the uid provided", func() {
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
					GIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
					},
				})).To(Succeed())

				Expect(fakeIDMapper.MapGIDsCallCount()).To(Equal(1))
				pid, mappings := fakeIDMapper.MapGIDsArgsForCall(0)

				Expect(pid).To(Equal(realTar.Process.Pid))
				Expect(mappings).To(Equal([]groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
				}))
			})

			Context("when it fails", func() {
				BeforeEach(func() {
					fakeIDMapper.MapGIDsReturns(errors.New("Boom!"))
				})

				It("should return an error", func() {
					Expect(tarCloner.Clone(logger, groot.CloneSpec{
						FromDir: fromDir,
						ToDir:   toDir,
						GIDMappings: []groot.IDMappingSpec{
							groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
						},
					})).To(MatchError("gid mapping: Boom!"))
				})
			})
		})
	})

	Context("when the image path does not exist", func() {
		It("should return an error", func() {
			Expect(tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: "/does/not/exist",
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring("image path `/does/not/exist` was not found")),
			)
		})
	})

	Context("when the image contains files that can only be read by root", func() {
		It("should return an error", func() {
			Expect(ioutil.WriteFile(path.Join(fromDir, "a-file"), []byte("hello-world"), 0000)).To(Succeed())

			Expect(tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: fromDir,
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring(fmt.Sprintf("reading from `%s`", fromDir))),
			)
		})
	})

	Context("when untarring fails for reasons", func() {
		BeforeEach(func() {
			toDir = "/random/dir"
		})

		It("should return an error", func() {
			Expect(tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: fromDir,
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring(fmt.Sprintf("writing to `%s`: %s", toDir, "exit status 2"))),
			)
		})
	})
})
