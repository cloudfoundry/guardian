package config_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builder", func() {
	var (
		configDir       string
		configFilePath  string
		builder         *config.Builder
		configStorePath string
	)

	BeforeEach(func() {
		configStorePath = "/hello"
	})

	JustBeforeEach(func() {
		var err error
		configDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cfg := config.Config{
			InsecureRegistries: []string{"http://example.org"},
			IgnoreBaseImages:   []string{"docker:///busybox"},
			BaseStorePath:      configStorePath,
		}

		configYaml, err := yaml.Marshal(cfg)
		Expect(err).NotTo(HaveOccurred())
		configFilePath = path.Join(configDir, "config.yaml")

		Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
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
			Expect(config.BaseStorePath).To(Equal("/hello"))
			Expect(config.UserBasedStorePath).To(Equal(filepath.Join("/hello", CurrentUserID)))
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

	Describe("WithStorePath", func() {
		Context("when provided store path and default store path are different", func() {
			It("overrides the config's store path entry with the provided store path with user ID postfix", func() {
				builder = builder.WithStorePath("/mnt/grootfs/data", "/var/lib/grootfs/data")
				config := builder.Build()
				Expect(config.UserBasedStorePath).To(Equal(filepath.Join("/mnt/grootfs/data", CurrentUserID)))
				Expect(config.BaseStorePath).To(Equal("/mnt/grootfs/data"))
			})
		})

		Context("when provided store path and default store path are the same", func() {
			It("uses the config's store path with user ID postfix", func() {
				builder = builder.WithStorePath("/var/lib/grootfs/data", "/var/lib/grootfs/data")
				config := builder.Build()
				Expect(config.UserBasedStorePath).To(Equal(filepath.Join("/hello", CurrentUserID)))
				Expect(config.BaseStorePath).To(Equal("/hello"))
			})

		})

		Context("when config doesn't have a store path property", func() {
			BeforeEach(func() {
				configStorePath = ""
			})

			It("uses the provided store path with user ID postfix", func() {
				builder = builder.WithStorePath("/var/lib/grootfs/data", "/var/lib/grootfs/data")
				config := builder.Build()
				Expect(config.UserBasedStorePath).To(Equal(filepath.Join("/var/lib/grootfs/data", CurrentUserID)))
				Expect(config.BaseStorePath).To(Equal("/var/lib/grootfs/data"))
			})
		})
	})
})
