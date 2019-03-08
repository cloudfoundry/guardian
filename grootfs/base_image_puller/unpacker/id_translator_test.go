package unpacker_test

import (
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/groot"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Noop ID Translator", func() {
	var translator unpacker.IDTranslator

	BeforeEach(func() {
		translator = unpacker.NewNoopIDTranslator()
	})

	It("maps the uid", func() {
		Expect(translator.TranslateUID(1234)).To(Equal(1234))
	})

	It("maps the gid", func() {
		Expect(translator.TranslateUID(9876)).To(Equal(9876))
	})

})

var _ = Describe("Mapping ID Translator", func() {
	var (
		uidMappings []groot.IDMappingSpec
		gidMappings []groot.IDMappingSpec

		translator unpacker.IDTranslator
	)
	BeforeEach(func() {
		uidMappings = []groot.IDMappingSpec{
			groot.IDMappingSpec{HostID: 1000, NamespaceID: 1500, Size: 2},
			groot.IDMappingSpec{HostID: 1100, NamespaceID: 0, Size: 1},
		}

		gidMappings = []groot.IDMappingSpec{
			groot.IDMappingSpec{HostID: 2000, NamespaceID: 2500, Size: 2},
			groot.IDMappingSpec{HostID: 2200, NamespaceID: 0, Size: 1},
		}
	})

	JustBeforeEach(func() {
		translator = unpacker.NewIDTranslator(uidMappings, gidMappings)
	})

	It("translates the root UID", func() {
		Expect(translator.TranslateUID(0)).To(Equal(1100))
	})

	It("translates non-root uid", func() {
		Expect(translator.TranslateUID(1501)).To(Equal(2500))
	})

	It("does not translate uid that is not mapped", func() {
		Expect(translator.TranslateUID(1010)).To(Equal(1010))
	})

	It("translates the root GID", func() {
		Expect(translator.TranslateGID(0)).To(Equal(2200))
	})

	It("translates non-root gid", func() {
		Expect(translator.TranslateGID(2501)).To(Equal(4500))
	})

	It("does not translate gid that is not mapped", func() {
		Expect(translator.TranslateGID(2010)).To(Equal(2010))
	})
})
