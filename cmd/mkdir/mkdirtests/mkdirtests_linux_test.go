package mkdirtests_test

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Mkdirtests", func() {

	Describe("normal usage", func() {

		var (
			tmpDir         string
			dirToCreate    string
			stdout, stderr io.Writer
			runerr         error
			cmd            *exec.Cmd
			user           string
			group          string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "mkdir")
			Expect(err).NotTo(HaveOccurred())

			dirToCreate = filepath.Join(tmpDir, "myNewDir")
			stdout = GinkgoWriter
			stderr = GinkgoWriter
			runerr = nil
			user = "0"
			group = "0"

		})

		JustBeforeEach(func() {
			commandArgs := []string{"-u", user, "-g", group, dirToCreate}
			cmd = exec.Command(mkdirbinpath, commandArgs...)
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			runerr = cmd.Run()
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("creates a valid directory path", func() {
			Expect(runerr).NotTo(HaveOccurred())
			Expect(dirToCreate).To(BeADirectory())
		})

		It("sets default directory permissions", func() {
			Expect(runerr).NotTo(HaveOccurred())
			stat, err := os.Stat(dirToCreate)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))
		})

		When("uid / gid differ from 0", func() {
			BeforeEach(func() {
				user = "42"
				group = "123"
			})

			It("sets the user and group on the directory", func() {
				Expect(runerr).NotTo(HaveOccurred())
				stat, err := os.Stat(dirToCreate)
				Expect(err).NotTo(HaveOccurred())
				sysStat, ok := stat.Sys().(*syscall.Stat_t)
				Expect(ok).To(BeTrue(), "couldn't make a unix.Stat_t out of os.FileInfo")
				Expect(sysStat.Uid).To(Equal(uint32(42)))
				Expect(sysStat.Gid).To(Equal(uint32(123)))
			})

			When("a parent directory already exists", func() {
				var (
					parentDir string
				)

				BeforeEach(func() {
					parentDir = filepath.Join(tmpDir, "parentDir")
					Expect(os.MkdirAll(parentDir, 0644)).To(Succeed())
					Expect(os.Chown(parentDir, 100, 200)).To(Succeed())
					dirToCreate = filepath.Join(parentDir, "anotherDir")
				})

				It("succeeds", func() {
					Expect(runerr).NotTo(HaveOccurred())
				})

				It("does not change the parent directory ownership", func() {
					stat, err := os.Stat(parentDir)
					Expect(err).NotTo(HaveOccurred())
					sysStat, ok := stat.Sys().(*syscall.Stat_t)
					Expect(ok).To(BeTrue())
					Expect(sysStat.Uid).To(Equal(uint32(100)))
					Expect(sysStat.Gid).To(Equal(uint32(200)))
				})

				It("does not change the parent directory permissions", func() {
					stat, err := os.Stat(parentDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0644)))
				})
			})

		})

		When("parent directories must be created", func() {
			var (
				parentDir string
			)

			BeforeEach(func() {
				parentDir = filepath.Join(tmpDir, "parentDir")
				dirToCreate = filepath.Join(parentDir, "anotherDir")
				user = "42"
				group = "123"
			})

			It("creates them", func() {
				Expect(runerr).NotTo(HaveOccurred())
				Expect(parentDir).To(BeADirectory())
				Expect(dirToCreate).To(BeADirectory())
			})

			It("sets the default perms on all created dirs", func() {
				Expect(runerr).NotTo(HaveOccurred())
				stat, err := os.Stat(parentDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))
			})

			It("sets the user and group ownership on the parent directory", func() {
				stat, err := os.Stat(parentDir)
				Expect(err).NotTo(HaveOccurred())
				sysStat, ok := stat.Sys().(*syscall.Stat_t)
				Expect(ok).To(BeTrue())
				Expect(sysStat.Uid).To(Equal(uint32(42)))
				Expect(sysStat.Gid).To(Equal(uint32(123)))
			})

		})

		When("the path contains a symlink", func() {
			var (
				parentDir string
			)

			BeforeEach(func() {
				parentDir = filepath.Join(tmpDir, "parentDir")
				Expect(os.Mkdir(parentDir, 0755)).To(Succeed())
				linkToParent := filepath.Join(tmpDir, "linkToParent")
				Expect(os.Symlink(parentDir, linkToParent)).To(Succeed())
				dirToCreate = filepath.Join(linkToParent, "anotherDir")
			})

			It("creates them", func() {
				Expect(runerr).NotTo(HaveOccurred())
				Expect(dirToCreate).To(BeADirectory())
			})

		})
	})

	Describe("validation", func() {
		var (
			stderr *gbytes.Buffer
			runerr error
			cmd    *exec.Cmd
		)

		BeforeEach(func() {
			stderr = gbytes.NewBuffer()
		})

		JustBeforeEach(func() {
			runerr = cmd.Run()
		})

		When("path is an empty string", func() {
			BeforeEach(func() {
				cmd = exec.Command(mkdirbinpath, "")
				cmd.Stderr = stderr
			})

			It("fails", func() {
				Expect(runerr).To(HaveOccurred())
				Expect(stderr).To(gbytes.Say("usage"))
			})
		})

		When("no args are passed", func() {
			BeforeEach(func() {
				cmd = exec.Command(mkdirbinpath)
				cmd.Stderr = stderr
			})

			It("fails", func() {
				Expect(runerr).To(HaveOccurred())
				Expect(stderr).To(gbytes.Say("usage"))
			})
		})
	})
})
