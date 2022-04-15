package idmapper_test

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/idmapper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MaxValidUid", func() {
	var (
		workDir     string
		uidmapPath  string
		maxUID      int
		maxValidErr error
	)

	BeforeEach(func() {
		workDir = tempDir("", "")
		uidmapPath = filepath.Join(workDir, "uidmap")

		writeFile(uidmapPath, []byte("0 0 1\n12345 0 3\n44 0 1"), os.ModePerm)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(workDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		maxUID, maxValidErr = idmapper.IDMap(uidmapPath).MaxValid()
	})

	It("doesn't fail", func() {
		Expect(maxValidErr).NotTo(HaveOccurred())
	})

	It("returns the largest value", func() {
		Expect(maxUID).To(Equal(12347))
	})

	Context("when the file has no entries", func() {
		BeforeEach(func() {
			writeFile(uidmapPath, []byte{}, os.ModePerm)
		})

		It("doesn't fail", func() {
			Expect(maxValidErr).NotTo(HaveOccurred())
		})

		It("should return 0", func() {
			Expect(maxUID).To(Equal(0))
		})
	})

	Context("when a line is invalid", func() {
		BeforeEach(func() {
			writeFile(uidmapPath, []byte("cake"), os.ModePerm)
		})

		It("returns an error", func() {
			Expect(maxValidErr).To(MatchError(`expected integer while parsing line "cake"`))
		})
	})
})
