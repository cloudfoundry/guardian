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
		cfg            config.Config
		createCfg      config.Create
		cleanCfg       config.Clean
		configDir      string
		configFilePath string
		builder        *config.Builder
	)

	BeforeEach(func() {
		createCfg = config.Create{
			WithClean:             false,
			WithoutMount:          false,
			ExcludeImageFromQuota: true,
			SkipLayerValidation:   true,
			InsecureRegistries:    []string{"http://example.org"},
			DiskLimitSizeBytes:    int64(1000),
		}

		cleanCfg = config.Clean{
			ThresholdBytes: int64(0),
		}

		cfg = config.Config{
			Create:         createCfg,
			Clean:          cleanCfg,
			StorePath:      "/hello",
			FSDriver:       "kitten-fs",
			DraxBin:        "/config/drax",
			TardisBin:      "/config/tardis",
			BtrfsProgsPath: "/config/btrfs-progs",
			NewuidmapBin:   "/config/newuidmap",
			NewgidmapBin:   "/config/newgidmap",
			MetronEndpoint: "config_endpoint:1111",
			LogLevel:       "info",
			LogFile:        "/path/to/a/file",
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
			Expect(config.Create.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			Expect(config.StorePath).To(Equal("/hello"))
		})

		Context("when disk limit property is invalid", func() {
			BeforeEach(func() {
				cfg.Create.DiskLimitSizeBytes = int64(-1)
			})

			It("returns an error", func() {
				_, err := builder.Build()
				Expect(err).To(MatchError("invalid argument: disk limit cannot be negative"))
			})
		})

		Context("when clean threshold property is invalid", func() {
			BeforeEach(func() {
				cfg.Clean.ThresholdBytes = int64(-1)
			})

			It("returns an error", func() {
				_, err := builder.Build()
				Expect(err).To(MatchError("invalid argument: clean threshold cannot be negative"))
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
			Expect(config.Create.InsecureRegistries).To(Equal([]string{"1", "2"}))
		})

		Context("when empty", func() {
			It("doesn't override the config's InsecureRegistries entry", func() {
				builder = builder.WithInsecureRegistries([]string{})
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Create.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			})
		})

		Context("when nil", func() {
			It("doesn't override the config's InsecureRegistries entry", func() {
				builder = builder.WithInsecureRegistries(nil)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Create.InsecureRegistries).To(Equal([]string{"http://example.org"}))
			})
		})
	})

	Describe("WithStorePath", func() {
		It("overrides the config's store path entry when command line flag is set", func() {
			builder = builder.WithStorePath("/mnt/grootfs/data", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.StorePath).To(Equal("/mnt/grootfs/data"))
		})

		Context("when store path is not provided via command line", func() {
			It("uses the config's store path ", func() {
				builder = builder.WithStorePath("/mnt/grootfs/data", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.StorePath).To(Equal("/hello"))
			})

			Context("and store path is not set in the config", func() {
				BeforeEach(func() {
					cfg.StorePath = ""
				})

				It("uses the provided store path ", func() {
					builder = builder.WithStorePath("/mnt/grootfs/data", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.StorePath).To(Equal("/mnt/grootfs/data"))
				})
			})
		})
	})

	Describe("WithFSDriver", func() {
		It("overrides the config's filesystem driver entry when command line flag is set", func() {
			builder = builder.WithFSDriver("dinosaur-fs", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.FSDriver).To(Equal("dinosaur-fs"))
		})

		Context("when filesystem driver is not provided via command line", func() {
			It("uses the config's filesystem driver", func() {
				builder = builder.WithFSDriver("dinosaur-fs", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.FSDriver).To(Equal("kitten-fs"))
			})

			Context("and filesystem driver is not set in the config", func() {
				BeforeEach(func() {
					cfg.FSDriver = ""
				})

				It("uses the provided filesystem driver", func() {
					builder = builder.WithFSDriver("dinosaur-fs", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.FSDriver).To(Equal("dinosaur-fs"))
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

	Describe("WithTardisBin", func() {
		It("overrides the config's tardis path entry when command line flag is set", func() {
			builder = builder.WithTardisBin("/my/tardis", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.TardisBin).To(Equal("/my/tardis"))
		})

		Context("when tardis path is not provided via command line", func() {
			It("uses the config's tardis path ", func() {
				builder = builder.WithTardisBin("/my/tardis", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.TardisBin).To(Equal("/config/tardis"))
			})

			Context("and tardis path is not set in the config", func() {
				BeforeEach(func() {
					cfg.TardisBin = ""
				})

				It("uses the provided tardis path ", func() {
					builder = builder.WithTardisBin("/my/tardis", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.TardisBin).To(Equal("/my/tardis"))
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

	Describe("WithBtrfsProgsPath", func() {
		It("overrides the config's btrfs path entry when command line flag is set", func() {
			builder = builder.WithBtrfsProgsPath("/my/btrfs-progs", true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.BtrfsProgsPath).To(Equal("/my/btrfs-progs"))
		})

		Context("when btrfs path is not provided via command line", func() {
			It("uses the config's btrfs path ", func() {
				builder = builder.WithBtrfsProgsPath("/my/btrfs-progs", false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.BtrfsProgsPath).To(Equal("/config/btrfs-progs"))
			})

			Context("and btrfs path is not set in the config", func() {
				BeforeEach(func() {
					cfg.BtrfsProgsPath = ""
				})

				It("uses the provided btrfs path ", func() {
					builder = builder.WithBtrfsProgsPath("/my/btrfs-progs", false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.BtrfsProgsPath).To(Equal("/my/btrfs-progs"))
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

	Describe("WithDiskLimitSizeBytes", func() {
		It("overrides the config's DiskLimitSizeBytes entry when flag is set", func() {
			diskLimit := int64(3000)
			builder = builder.WithDiskLimitSizeBytes(diskLimit, true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Create.DiskLimitSizeBytes).To(Equal(diskLimit))
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithDiskLimitSizeBytes(10, false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Create.DiskLimitSizeBytes).To(Equal(cfg.Create.DiskLimitSizeBytes))
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

	Describe("WithExcludeImageFromQuota", func() {
		It("overrides the config's ExcludeImageFromQuota when the flag is set", func() {
			builder = builder.WithExcludeImageFromQuota(false, true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Create.ExcludeImageFromQuota).To(BeFalse())
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithExcludeImageFromQuota(false, false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Create.ExcludeImageFromQuota).To(BeTrue())
			})
		})
	})

	Describe("WithSkipLayerValidation", func() {
		It("overrides the config's SkipLayerValidation when the flag is set", func() {
			builder = builder.WithSkipLayerValidation(false, true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Create.SkipLayerValidation).To(BeFalse())
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithSkipLayerValidation(false, false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Create.SkipLayerValidation).To(BeTrue())
			})
		})
	})

	Describe("WithCleanThresholdBytes", func() {
		It("overrides the config's CleanThresholdBytes entry when the flag is set", func() {
			builder = builder.WithCleanThresholdBytes(1024, true)
			config, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Clean.ThresholdBytes).To(Equal(int64(1024)))
		})

		Context("when flag is not set", func() {
			It("uses the config entry", func() {
				builder = builder.WithCleanThresholdBytes(1024, false)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Clean.ThresholdBytes).To(Equal(cfg.Clean.ThresholdBytes))
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
			Context("when no-clean is set, and clean is not set", func() {
				BeforeEach(func() {
					cfg.Create.WithClean = true
				})

				It("overrides the config's entry", func() {
					builder = builder.WithClean(false, true)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Create.WithClean).To(BeFalse())
				})
			})

			Context("when no-clean is not set, and clean is set", func() {
				BeforeEach(func() {
					cfg.Create.WithClean = false
				})

				It("overrides the config's entry", func() {
					builder = builder.WithClean(true, false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Create.WithClean).To(BeTrue())
				})
			})

			Context("when no-clean is not set, and clean is not set", func() {
				BeforeEach(func() {
					cfg.Create.WithClean = true
				})

				It("uses the config value", func() {
					cfg.Create.WithClean = true
					builder = builder.WithClean(false, false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Create.WithClean).To(BeTrue())
				})
			})
		})

		Describe("WithMount", func() {
			Context("when without-mount is set, and with-mount is not set", func() {
				BeforeEach(func() {
					cfg.Create.WithoutMount = false
				})

				It("overrides the config's entry", func() {
					builder = builder.WithMount(false, true)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Create.WithoutMount).To(BeTrue())
				})
			})

			Context("when without-mount is not set, and with-mount is set", func() {
				BeforeEach(func() {
					cfg.Create.WithoutMount = true
				})

				It("overrides the config's entry", func() {
					builder = builder.WithMount(true, false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Create.WithoutMount).To(BeFalse())
				})
			})

			Context("when without-mount is not set, and with-mount is not set", func() {
				BeforeEach(func() {
					cfg.Create.WithoutMount = false
				})

				It("uses the config value", func() {
					builder = builder.WithMount(false, false)
					config, err := builder.Build()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Create.WithoutMount).To(BeFalse())
				})
			})
		})

		Describe("WithStoreSizeBytes", func() {
			It("sets the correct config value", func() {
				builder = builder.WithStoreSizeBytes(1024)
				config, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Init.StoreSizeBytes).To(Equal(int64(1024)))
			})
		})
	})
})
