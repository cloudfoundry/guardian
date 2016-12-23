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
		cfg            config.Config
		configDir      string
		configFilePath string
		builder        *config.Builder
	)

	BeforeEach(func() {
		cfg = config.Config{
			BaseStorePath:             "/hello",
			DraxBin:                   "/config/drax",
			BtrfsBin:                  "/config/btrfs",
			NewuidmapBin:              "/config/newuidmap",
			NewgidmapBin:              "/config/newgidmap",
			MetronEndpoint:            "config_endpoint:1111",
			UIDMappings:               []string{"config-uid-mapping"},
			GIDMappings:               []string{"config-gid-mapping"},
			IgnoreBaseImages:          []string{"docker:///busybox"},
			InsecureRegistries:        []string{"http://example.org"},
			DiskLimitSizeBytes:        int64(1000),
			ExcludeBaseImageFromQuota: true,
			CleanThresholdBytes:       int64(0),
			CleanOnCreate:             false,
			LogLevel:                  "info",
			LogFile:                   "/path/to/a/file",
		}
	})

	JustBeforeEach(func() {
		var err error
		configDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

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
				cfg.DiskLimitSizeBytes = int64(-1)
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
					cfg.BaseStorePath = ""
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
					cfg.DraxBin = ""
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

	Describe("WithNewuidmapBin", func() {
		It("overrides the config's newuidmap path entry when command line flag is set", func() {
			builder = builder.WithNewuidmapBin("/my/newuidmap", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.NewuidmapBin).To(Equal("/my/newuidmap"))
		})

		Context("when newuidmap path is not provided via command line", func() {
			It("uses the config's newuidmap path ", func() {
				builder = builder.WithNewuidmapBin("/my/newuidmap", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.NewuidmapBin).To(Equal("/config/newuidmap"))
			})

			Context("and newuidmap path is not set in the config", func() {
				BeforeEach(func() {
					cfg.NewuidmapBin = ""
				})

				It("uses the provided newuidmap path ", func() {
					builder = builder.WithNewuidmapBin("/my/newuidmap", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.NewuidmapBin).To(Equal("/my/newuidmap"))
				})
			})
		})
	})

	Describe("WithNewgidmapBin", func() {
		It("overrides the config's newgidmap path entry when command line flag is set", func() {
			builder = builder.WithNewgidmapBin("/my/newgidmap", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.NewgidmapBin).To(Equal("/my/newgidmap"))
		})

		Context("when newgidmap path is not provided via command line", func() {
			It("uses the config's newgidmap path ", func() {
				builder = builder.WithNewgidmapBin("/my/newgidmap", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.NewgidmapBin).To(Equal("/config/newgidmap"))
			})

			Context("and newgidmap path is not set in the config", func() {
				BeforeEach(func() {
					cfg.NewgidmapBin = ""
				})

				It("uses the provided newgidmap path ", func() {
					builder = builder.WithNewgidmapBin("/my/newgidmap", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.NewgidmapBin).To(Equal("/my/newgidmap"))
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
					cfg.BtrfsBin = ""
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
				Expect(config.DiskLimitSizeBytes).To(Equal(cfg.DiskLimitSizeBytes))
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
			builder = builder.WithCleanThresholdBytes(1024, true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.CleanThresholdBytes).To(Equal(int64(1024)))
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithCleanThresholdBytes(1024, false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.CleanThresholdBytes).To(Equal(cfg.CleanThresholdBytes))
			})
		})
	})

	Describe("WithLogLevel", func() {
		It("overrides the config's Log Level entry", func() {
			builder = builder.WithLogLevel("debug", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.LogLevel).To(Equal("debug"))
		})

		Context("when empty", func() {
			It("doesn't override the config's log level entry", func() {
				builder = builder.WithLogLevel("", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.LogLevel).To(Equal(cfg.LogLevel))
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
				Expect(config.LogFile).To(Equal(cfg.LogFile))
			})
		})

		Describe("WithCleanOnCreate", func() {
			It("overrides the config's entry when the flag is set", func() {
				builder = builder.WithCleanOnCreate("true", true)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.CleanOnCreate).To(BeTrue())
			})

			It("overrides the config's entry when the flag is set", func() {
				builder = builder.WithCleanOnCreate("false", true)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.CleanOnCreate).To(BeFalse())
			})

			Context("when flag is not valid boolean", func() {
				It("sets the flag to false", func() {
					builder = builder.WithCleanOnCreate("not-valid-boolean", true)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.CleanOnCreate).To(BeFalse())
				})
			})
		})
	})
})
