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
		configDir                   string
		configFilePath              string
		builder                     *config.Builder
		configStorePath             string
		configDraxBin               string
		configBtrfsBin              string
		configMetronEndpoint        string
		configUIDMappings           []string
		configGIDMappings           []string
		configDiskLimitSizeBytes    int64
		configExcludeImageFromQuota bool
		configCleanThresholdBytes   uint64
		configLogLevel              string
		configLogFile               string
	)

	BeforeEach(func() {
		configStorePath = "/hello"
		configDraxBin = "/config/drax"
		configBtrfsBin = "/config/btrfs"
		configMetronEndpoint = "config_endpoint:1111"
		configUIDMappings = []string{"config-uid-mapping"}
		configGIDMappings = []string{"config-gid-mapping"}
		configDiskLimitSizeBytes = int64(1000)
		configExcludeImageFromQuota = true
		configCleanThresholdBytes = 0
		configLogLevel = "info"
		configLogFile = "/path/to/a/file"
	})

	JustBeforeEach(func() {
		var err error
		configDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cfg := config.Config{
			InsecureRegistries:        []string{"http://example.org"},
			IgnoreBaseImages:          []string{"docker:///busybox"},
			BaseStorePath:             configStorePath,
			DraxBin:                   configDraxBin,
			BtrfsBin:                  configBtrfsBin,
			MetronEndpoint:            configMetronEndpoint,
			UIDMappings:               configUIDMappings,
			GIDMappings:               configGIDMappings,
			DiskLimitSizeBytes:        configDiskLimitSizeBytes,
			ExcludeBaseImageFromQuota: configExcludeImageFromQuota,
			CleanThresholdBytes:       configCleanThresholdBytes,
			LogLevel:                  configLogLevel,
			LogFile:                   configLogFile,
		}

		configYaml, err := yaml.Marshal(cfg)
		Expect(err).NotTo(HaveOccurred())
		configFilePath = path.Join(configDir, "config.yaml")

		Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
		builder, err = config.NewBuilder(configFilePath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(configDir)).To(Succeed())
	})

	Describe("Build", func() {
		It("returns the values read from the config yaml", func() {
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			Expect(config.IgnoreBaseImages).To(Equal([]string{"docker:///busybox"}))
			Expect(config.BaseStorePath).To(Equal("/hello"))
		})

		Context("when disk limit property is invalid", func() {
			BeforeEach(func() {
				configDiskLimitSizeBytes = int64(-1)
			})

			It("returns an error", func() {
				_, err := builder.Build()
				Expect(err).To(MatchError("invalid argument: disk limit cannot be negative"))
			})
		})

		Context("when config is invalid", func() {
			JustBeforeEach(func() {
				configFilePath = path.Join(configDir, "invalid_config.yaml")
				Expect(ioutil.WriteFile(configFilePath, []byte("foo-bar"), 0755)).To(Succeed())

			})

			It("returns an error", func() {
				_, err := config.NewBuilder(configFilePath)
				Expect(err).To(MatchError(ContainSubstring("invalid config file")))
			})
		})
	})

	Describe("WithInsecureRegistries", func() {
		It("overrides the config's InsecureRegistries entry", func() {
			builder = builder.WithInsecureRegistries([]string{"1", "2"})
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.InsecureRegistries).To(Equal([]string{"1", "2"}))
		})

		Context("when empty", func() {
			It("doesn't override the config's InsecureRegistries entry", func() {
				builder = builder.WithInsecureRegistries([]string{})
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			})
		})

		Context("when nil", func() {
			It("doesn't override the config's InsecureRegistries entry", func() {
				builder = builder.WithInsecureRegistries(nil)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			})
		})
	})

	Describe("WithIgnoreBaseImages", func() {
		It("overrides the config's IgnoreBaseImages entry", func() {
			builder = builder.WithIgnoreBaseImages([]string{"1", "2"})
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.IgnoreBaseImages).To(Equal([]string{"1", "2"}))
		})

		Context("when empty", func() {
			It("doesn't override the config's IgnoreBaseImages entry", func() {
				builder = builder.WithIgnoreBaseImages([]string{})
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.IgnoreBaseImages).To(Equal([]string{"docker:///busybox"}))
			})
		})

		Context("when nil", func() {
			It("doesn't override the config's IgnoreBaseImages entry", func() {
				builder = builder.WithIgnoreBaseImages(nil)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.IgnoreBaseImages).To(Equal([]string{"docker:///busybox"}))
			})
		})
	})

	Describe("WithStorePath", func() {
		It("overrides the config's store path entry when command line flag is set", func() {
			builder = builder.WithStorePath("/mnt/grootfs/data", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.UserBasedStorePath).To(Equal(filepath.Join("/mnt/grootfs/data", CurrentUserID)))
			Expect(config.BaseStorePath).To(Equal("/mnt/grootfs/data"))
		})

		Context("when store path is not provided via command line", func() {
			It("uses the config's store path ", func() {
				builder = builder.WithStorePath("/mnt/grootfs/data", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.UserBasedStorePath).To(Equal(filepath.Join("/hello/", CurrentUserID)))
				Expect(config.BaseStorePath).To(Equal("/hello"))
			})

			Context("and store path is not set in the config", func() {
				BeforeEach(func() {
					configStorePath = ""
				})

				It("uses the provided store path ", func() {
					builder = builder.WithStorePath("/mnt/grootfs/data", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.UserBasedStorePath).To(Equal(filepath.Join("/mnt/grootfs/data", CurrentUserID)))
					Expect(config.BaseStorePath).To(Equal("/mnt/grootfs/data"))
				})
			})
		})
	})

	Describe("WithDraxBin", func() {
		It("overrides the config's drax path entry when command line flag is set", func() {
			builder = builder.WithDraxBin("/my/drax", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.DraxBin).To(Equal("/my/drax"))
		})

		Context("when drax path is not provided via command line", func() {
			It("uses the config's drax path ", func() {
				builder = builder.WithDraxBin("/my/drax", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.DraxBin).To(Equal("/config/drax"))
			})

			Context("and drax path is not set in the config", func() {
				BeforeEach(func() {
					configDraxBin = ""
				})

				It("uses the provided drax path ", func() {
					builder = builder.WithDraxBin("/my/drax", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.DraxBin).To(Equal("/my/drax"))
				})
			})
		})
	})

	Describe("WithBtrfsBin", func() {
		It("overrides the config's btrfs path entry when command line flag is set", func() {
			builder = builder.WithBtrfsBin("/my/btrfs", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.BtrfsBin).To(Equal("/my/btrfs"))
		})

		Context("when btrfs path is not provided via command line", func() {
			It("uses the config's btrfs path ", func() {
				builder = builder.WithBtrfsBin("/my/btrfs", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.BtrfsBin).To(Equal("/config/btrfs"))
			})

			Context("and btrfs path is not set in the config", func() {
				BeforeEach(func() {
					configBtrfsBin = ""
				})

				It("uses the provided btrfs path ", func() {
					builder = builder.WithBtrfsBin("/my/btrfs", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.BtrfsBin).To(Equal("/my/btrfs"))
				})
			})
		})
	})

	Describe("WithMetronEndpoint", func() {
		It("overrides the config's metron endpoint entry", func() {
			builder = builder.WithMetronEndpoint("127.0.0.1:5555")
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.MetronEndpoint).To(Equal("127.0.0.1:5555"))
		})

		Context("when empty", func() {
			It("doesn't override the config's metron endpoint entry", func() {
				builder = builder.WithMetronEndpoint("")
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.MetronEndpoint).To(Equal("config_endpoint:1111"))
			})
		})
	})

	Describe("WithUIDMappings", func() {
		It("overrides the config's UIDMappings entry", func() {
			builder = builder.WithUIDMappings([]string{"1", "2"})
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.UIDMappings).To(Equal([]string{"1", "2"}))
		})

		Context("when empty", func() {
			It("doesn't override the config's UIDMappings entry", func() {
				builder = builder.WithUIDMappings([]string{})
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.UIDMappings).To(Equal([]string{"config-uid-mapping"}))
			})
		})

		Context("when nil", func() {
			It("doesn't override the config's UIDMappings entry", func() {
				builder = builder.WithUIDMappings(nil)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.UIDMappings).To(Equal([]string{"config-uid-mapping"}))
			})
		})
	})

	Describe("WithGIDMappings", func() {
		It("overrides the config's GIDMappings entry", func() {
			builder = builder.WithGIDMappings([]string{"1", "2"})
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.GIDMappings).To(Equal([]string{"1", "2"}))
		})

		Context("when empty", func() {
			It("doesn't override the config's GIDMappings entry", func() {
				builder = builder.WithGIDMappings([]string{})
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.GIDMappings).To(Equal([]string{"config-gid-mapping"}))
			})
		})

		Context("when nil", func() {
			It("doesn't override the config's GIDMappings entry", func() {
				builder = builder.WithGIDMappings(nil)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.GIDMappings).To(Equal([]string{"config-gid-mapping"}))
			})
		})
	})

	Describe("WithDiskLimitSizeBytes", func() {
		It("overrides the config's DiskLimitSizeBytes entry when flag is set", func() {
			diskLimit := int64(3000)
			builder = builder.WithDiskLimitSizeBytes(diskLimit, true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.DiskLimitSizeBytes).To(Equal(diskLimit))
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithDiskLimitSizeBytes(10, false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.DiskLimitSizeBytes).To(Equal(configDiskLimitSizeBytes))
			})
		})

		Context("when negative", func() {
			It("returns an error", func() {
				diskLimit := int64(-300)
				builder = builder.WithDiskLimitSizeBytes(diskLimit, true)
				_, err := builder.Build()
				Expect(err).To(MatchError("invalid argument: disk limit cannot be negative"))
			})
		})
	})

	Describe("WithExcludeBaseImageFromQuota", func() {
		It("overrides the config's ExcludeBaseImageFromQuota when the flag is set", func() {
			builder = builder.WithExcludeBaseImageFromQuota(false, true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ExcludeBaseImageFromQuota).To(BeFalse())
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithExcludeBaseImageFromQuota(false, false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.ExcludeBaseImageFromQuota).To(BeTrue())
			})
		})
	})

	Describe("WithCleanThresholdBytes", func() {
		It("overrides the config's CleanThresholdBytes entry when the flag is set", func() {
			builder = builder.WithCleanThresholdBytes(uint64(1024), true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.CleanThresholdBytes).To(Equal(uint64(1024)))
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithCleanThresholdBytes(uint64(1024), false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.CleanThresholdBytes).To(Equal(configCleanThresholdBytes))
			})
		})
	})

	Describe("WithLogLevel", func() {
		It("overrides the config's Log Level entry", func() {
			builder = builder.WithLogLevel("debug")
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.LogLevel).To(Equal("debug"))
		})

		Context("when empty", func() {
			It("doesn't override the config's log level entry", func() {
				builder = builder.WithLogLevel("")
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.LogLevel).To(Equal(configLogLevel))
			})
		})
	})

	Describe("WithLogFile", func() {
		It("overrides the config's Log File entry", func() {
			builder = builder.WithLogFile("/path/to/log-file.log")
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.LogFile).To(Equal("/path/to/log-file.log"))
		})

		Context("when empty", func() {
			It("doesn't override the config's log file entry", func() {
				builder = builder.WithLogFile("")
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.LogFile).To(Equal(configLogFile))
			})
		})
	})
})
