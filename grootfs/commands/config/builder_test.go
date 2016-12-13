package config_test

import (
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/commands/config"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builder", func() {
	var (
		configDir      string
		configFilePath string
		builder        *config.Builder
	)

	BeforeEach(func() {
		var err error
		configDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cfg := config.Config{
			InsecureRegistries: []string{"http://example.org"},
			IgnoreBaseImages:   []string{"docker:///busybox"},
		}

		configYaml, err := yaml.Marshal(cfg)
		Expect(err).NotTo(HaveOccurred())
		configFilePath = path.Join(configDir, "config.yaml")

		Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
	})

	JustBeforeEach(func() {
		var err error
		builder, err = config.NewBuilderFromFile(configFilePath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(configDir)).To(Succeed())
	})

	Describe("Build", func() {
		It("returns the values read from the config yaml", func() {
			config := builder.Build()
			Expect(config.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			Expect(config.IgnoreBaseImages).To(Equal([]string{"docker:///busybox"}))
		})
	})

	Describe("WithInsecureRegistries", func() {
		It("overrides the config's InsecureRegistries entry", func() {
			builder = builder.WithInsecureRegistries([]string{"1", "2"})
			config := builder.Build()
			Expect(config.InsecureRegistries).To(Equal([]string{"1", "2"}))
		})

		Context("when empty", func() {
			It("doesn't override the config's InsecureRegistries entry", func() {
				builder = builder.WithInsecureRegistries([]string{})
				config := builder.Build()
				Expect(config.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			})
		})

		Context("when nil", func() {
			It("doesn't override the config's InsecureRegistries entry", func() {
				builder = builder.WithInsecureRegistries(nil)
				config := builder.Build()
				Expect(config.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			})
		})
	})

	Describe("WithIgnoreBaseImages", func() {
		It("overrides the config's IgnoreBaseImages entry", func() {
			builder = builder.WithIgnoreBaseImages([]string{"1", "2"})
			config := builder.Build()
			Expect(config.IgnoreBaseImages).To(Equal([]string{"1", "2"}))
		})

		Context("when empty", func() {
			It("doesn't override the config's IgnoreBaseImages entry", func() {
				builder = builder.WithIgnoreBaseImages([]string{})
				config := builder.Build()
				Expect(config.IgnoreBaseImages).To(Equal([]string{"docker:///busybox"}))
			})
		})

		Context("when nil", func() {
			It("doesn't override the config's IgnoreBaseImages entry", func() {
				builder = builder.WithIgnoreBaseImages(nil)
				config := builder.Build()
				Expect(config.IgnoreBaseImages).To(Equal([]string{"docker:///busybox"}))
			})
		})
	})

})
