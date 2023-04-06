package unpacker_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NSIdMapperUnpacker", func() {
	var (
		reexecer *grootfakes.FakeSandboxReexecer
		unpacker *unpackerpkg.NSIdMapperUnpacker

		logger                    lager.Logger
		storePath                 string
		shouldCloneUserNsOnUnpack bool
		targetPath                string
	)

	BeforeEach(func() {
		var err error

		shouldCloneUserNsOnUnpack = false
		reexecer = new(grootfakes.FakeSandboxReexecer)
		reexecer.ReexecReturns([]byte("{\"BytesWritten\":1024}"), nil)

		logger = lagertest.NewTestLogger("test-store")

		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		targetPath = filepath.Join(storePath, "rootfs")
	})

	JustBeforeEach(func() {
		unpacker = unpackerpkg.NewNSIdMapperUnpacker(storePath, reexecer, shouldCloneUserNsOnUnpack, groot.IDMappings{})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	It("passes the rootfs path, base-directory and filesystem to the unpack command", func() {
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath:    targetPath,
			BaseDirectory: "/base-folder/",
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(reexecer.ReexecCallCount()).To(Equal(1))
		_, reexecSpec := reexecer.ReexecArgsForCall(0)

		Expect(reexecSpec.Args).To(Equal(
			[]string{".", "/base-folder/", "null", "null", strconv.FormatBool(!shouldCloneUserNsOnUnpack)},
		))
	})

	It("returns the unpack result", func() {
		unpackOutput, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(unpackOutput).To(Equal(base_image_puller.UnpackOutput{BytesWritten: 1024}))
	})

	It("does not clone user namespace", func() {
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(reexecer.ReexecCallCount()).To(Equal(1))
		_, reexecSpec := reexecer.ReexecArgsForCall(0)

		Expect(reexecSpec.CloneUserns).To(BeFalse())
	})

	Context("when asked to clone user namespace", func() {
		BeforeEach(func() {
			shouldCloneUserNsOnUnpack = true
		})

		It("clones user namespace", func() {
			unpacker.Unpack(logger, base_image_puller.UnpackSpec{})

			Expect(reexecer.ReexecCallCount()).To(Equal(1))
			_, reexecSpec := reexecer.ReexecArgsForCall(0)

			Expect(reexecSpec.CloneUserns).To(BeTrue())
		})
	})

	It("reexecs the unpack command", func() {
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(reexecer.ReexecCallCount()).To(Equal(1))
		reexecCmd, _ := reexecer.ReexecArgsForCall(0)
		Expect(reexecCmd).To(Equal("unpack"))
	})

	It("chroots into target path", func() {
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(reexecer.ReexecCallCount()).To(Equal(1))
		_, reexecSpec := reexecer.ReexecArgsForCall(0)

		Expect(reexecSpec.ChrootDir).To(Equal(targetPath))
	})

	It("sends stdin to reeexec", func() {
		stdinContent := "some stuff in stdin"
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:     ioutil.NopCloser(bytes.NewBufferString(stdinContent)),
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(reexecer.ReexecCallCount()).To(Equal(1))
		_, reexecSpec := reexecer.ReexecArgsForCall(0)

		streamContent, err := ioutil.ReadAll(reexecSpec.Stdin)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(streamContent)).To(Equal(stdinContent))
	})

	Context("when the unpack prints invalid output", func() {
		It("returns an error", func() {
			reexecer.ReexecReturns([]byte("abcd"), nil)
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).To(MatchError(ContainSubstring("invalid unpack output")))
		})
	})

	Context("when the reexecer fails", func() {
		BeforeEach(func() {
			reexecer.ReexecReturns([]byte("hello-world"), errors.New("reexec-error"))
		})

		It("returns an error", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).To(MatchError(ContainSubstring("reexec-error")))
		})

		It("returns the command output in the error message", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).To(MatchError(ContainSubstring("hello-world")))
		})
	})
})
