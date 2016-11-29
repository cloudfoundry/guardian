package config_test

import (
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/commands/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Load", func() {
	var (
		configDir      string
		configFilePath string
	)

	BeforeEach(func() {
		var err error
		configDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cfg := config.Config{
			InsecureRegistries: []string{"http://example.org"},
		}

		configYaml, err := yaml.Marshal(cfg)
		Expect(err).NotTo(HaveOccurred())
		configFilePath = path.Join(configDir, "config.yaml")

		Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(configDir)).To(Succeed())
	})

	It("loads a config file", func() {
		config, err := config.Load(configFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(config.InsecureRegistries).To(ConsistOf([]string{"http://example.org"}))
	})

	Context("when filepath is invalid", func() {
		It("returns an error", func() {
			_, err := config.Load("/tmp/not-here")
			Expect(err).To(MatchError(ContainSubstring("invalid config path")))
		})
	})

	Context("when config file has invalid content", func() {
		BeforeEach(func() {
			configFilePath = path.Join(configDir, "invalid-config.yaml")
			Expect(ioutil.WriteFile(configFilePath, []byte("invalid-content"), 0755)).To(Succeed())
		})

		It("returns an error", func() {
			_, err := config.Load(configFilePath)
			Expect(err).To(MatchError(ContainSubstring("invalid config file")))
		})
	})
})
