package removeall_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/grootfs/removeall"
)

var _ = Describe("Removeall", func() {
	var tmpDir string
	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "removeall")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("removes a regular file", func() {
		path := filepath.Join(tmpDir, "poptata.txt")
		_, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())

		Expect(RemoveAll(path)).To(Succeed())
		Expect(path).NotTo(BeAnExistingFile())
	})

	It("removes an empty directory", func() {
		dir := filepath.Join(tmpDir, "poptatadir")
		Expect(os.Mkdir(dir, os.ModePerm)).To(Succeed())

		Expect(RemoveAll(dir)).To(Succeed())
		Expect(dir).NotTo(BeADirectory())
	})

	It("removes an directory with files in it", func() {
		dir := filepath.Join(tmpDir, "poptatadir")
		Expect(os.Mkdir(dir, os.ModePerm)).To(Succeed())
		file1 := filepath.Join(dir, "poptata1.txt")
		_, err := os.Create(file1)
		Expect(err).NotTo(HaveOccurred())

		file2 := filepath.Join(dir, "poptata2.txt")
		_, err = os.Create(file2)
		Expect(err).NotTo(HaveOccurred())

		Expect(RemoveAll(dir)).To(Succeed())
		Expect(dir).NotTo(BeADirectory())
		Expect(file1).NotTo(BeAnExistingFile())
		Expect(file2).NotTo(BeAnExistingFile())
	})

	It("removes a nested directory with contents", func() {
		dir := filepath.Join(tmpDir, "poptatadir", "nestedpoptata")
		Expect(os.MkdirAll(dir, os.ModePerm)).To(Succeed())

		file1 := filepath.Join(dir, "poptata1.txt")
		_, err := os.Create(file1)
		Expect(err).NotTo(HaveOccurred())

		file2 := filepath.Join(dir, "poptata2.txt")
		_, err = os.Create(file2)
		Expect(err).NotTo(HaveOccurred())

		Expect(RemoveAll(dir)).To(Succeed())
		Expect(dir).NotTo(BeADirectory())

		Expect(file1).NotTo(BeAnExistingFile())
		Expect(file2).NotTo(BeAnExistingFile())
	})

	It("succeeds if the path does not exist", func() {
		Expect(RemoveAll("not/a/path")).To(Succeed())
	})

	It("can remove long paths", func() {
		dir := filepath.Join(tmpDir, "poptatadir")
		Expect(os.Mkdir(dir, os.ModePerm)).To(Succeed())
		createDirectories(dir, 50, 1, 100)
		Expect(RemoveAll(dir)).To(Succeed())
		Expect(dir).NotTo(BeADirectory())
	})

	It("can remove directories with many entries", func() {
		dir := filepath.Join(tmpDir, "poptatadir")
		Expect(os.Mkdir(dir, os.ModePerm)).To(Succeed())
		createDirectories(dir, 10, 1000, 100)
		Expect(RemoveAll(dir)).To(Succeed())
		Expect(dir).NotTo(BeADirectory())
	})

})

func createDirectories(baseDir string, width, height, namelength int) {
	prevWorkingDirectory, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	for h := 0; h < height; h++ {
		Expect(os.Chdir(baseDir)).To(Succeed())
		for w := 0; w < width; w++ {
			dirname := ""
			for n := 0; n < namelength; n++ {
				dirname = dirname + strconv.Itoa(h)
			}
			dirname = dirname[0:namelength]
			for {
				err := os.Mkdir(dirname, os.ModePerm)
				if os.IsExist(err) {
					dirname = dirname[0:len(dirname)-1] + strconv.Itoa(int(dirname[len(dirname)-1])+1)
				} else {
					break
				}
			}

			Expect(os.Chdir(dirname)).To(Succeed())
		}
	}

	os.Chdir(prevWorkingDirectory)
}
