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
		perms          string
		stdout, stderr io.Writer
		runerr         error
		args           []string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "mkdir")
		dirToCreate = filepath.Join(tmpDir, "myNewDir")
		perms = "0775"
		stdout = GinkgoWriter
		stderr = GinkgoWriter
		runerr = nil

		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		args = []string{}
		if dirToCreate != "" {
			args = append(args, "-path", dirToCreate)
		}
		if perms != "" {
			args = append(args, "-perm", perms)
		}

		cmd := exec.Command(mkdirbinpath, args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		runerr = cmd.Run()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	When("path isn't passed", func() {
		BeforeEach(func() {
			dirToCreate = ""
			stderr = gbytes.NewBuffer()
		})

		It("fails if no argument is passed", func() {
			Expect(runerr).To(HaveOccurred())
			Expect(stderr).To(gbytes.Say("usage"))
		})
	})

	When("valid args are passed", func() {
		BeforeEach(func() {
			perms = "0700"
		})

		It("creates a valid directory path", func() {
			Expect(runerr).NotTo(HaveOccurred())
			Expect(dirToCreate).To(BeADirectory())
		})

		It("sets directory permissions as given", func() {
			Expect(runerr).NotTo(HaveOccurred())
			stat, err := os.Stat(dirToCreate)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
		})
	})

	When("perms isn't passed", func() {
		BeforeEach(func() {
			perms = ""
		})

		It("sets directory permissions with the default 0755", func() {
			Expect(runerr).NotTo(HaveOccurred())
			stat, err := os.Stat(dirToCreate)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))
		})
	})

	When("parent directories must be created", func() {
		var (
			parentDir string
		)
		BeforeEach(func() {
			perms = "0750"
			parentDir = filepath.Join(tmpDir, "parentDir")
			dirToCreate = filepath.Join(parentDir, "anotherDir")
		})

		It("creates them", func() {
			Expect(runerr).NotTo(HaveOccurred())
			Expect(parentDir).To(BeADirectory())
			Expect(dirToCreate).To(BeADirectory())
		})

		It("sets the perms on all created dirs", func() {
			Expect(runerr).NotTo(HaveOccurred())
			stat, err := os.Stat(parentDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0750)))
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
