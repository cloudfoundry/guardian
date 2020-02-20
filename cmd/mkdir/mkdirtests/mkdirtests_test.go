package mkdirtests_test

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Mkdirtests", func() {

	var (
		tmpDir         string
		dirToCreate    string
		stdout, stderr io.Writer
		runerr         error
		cmd            *exec.Cmd
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "mkdir")
		Expect(err).NotTo(HaveOccurred())

		dirToCreate = filepath.Join(tmpDir, "myNewDir")
		stdout = GinkgoWriter
		stderr = GinkgoWriter
		runerr = nil

		cmd = exec.Command(mkdirbinpath, dirToCreate)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
	})

	JustBeforeEach(func() {
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

	When("path is an empty string", func() {
		BeforeEach(func() {
			cmd.Args[1] = ""
			cmd.Stderr = gbytes.NewBuffer()
		})

		It("fails", func() {
			Expect(runerr).To(HaveOccurred())
			Expect(cmd.Stderr).To(gbytes.Say("usage"))
		})
	})

	When("no args are passed", func() {
		BeforeEach(func() {
			cmd.Args = cmd.Args[:0]
			cmd.Stderr = gbytes.NewBuffer()
		})

		It("fails", func() {
			Expect(runerr).To(HaveOccurred())
			Expect(cmd.Stderr).To(gbytes.Say("usage"))
		})
	})

	When("parent directories must be created", func() {
		var (
			parentDir string
		)
		BeforeEach(func() {
			parentDir = filepath.Join(tmpDir, "parentDir")
			dirToCreate = filepath.Join(parentDir, "anotherDir")
			cmd.Args[1] = dirToCreate
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
			cmd.Args[1] = dirToCreate
		})

		It("creates them", func() {
			Expect(runerr).NotTo(HaveOccurred())
			Expect(dirToCreate).To(BeADirectory())
		})
	})
})
