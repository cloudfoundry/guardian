package config

import (
	"io/ioutil"

	errorspkg "github.com/pkg/errors"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	StorePath      string `yaml:"store"`
	FSDriver       string `yaml:"driver"`
	DraxBin        string `yaml:"drax_bin"`
	BtrfsBin       string `yaml:"btrfs_bin"`
	NewuidmapBin   string `yaml:"newuidmap_bin"`
	NewgidmapBin   string `yaml:"newgidmap_bin"`
	MetronEndpoint string `yaml:"metron_endpoint"`
	LogLevel       string `yaml:"log_level"`
	LogFile        string `yaml:"log_file"`
	Create         Create `yaml:"create"`
	Clean          Clean  `yaml:"clean"`
}

type Create struct {
	ExcludeImageFromQuota bool     `yaml:"exclude_image_from_quota"`
	WithClean             bool     `yaml:"with_clean"`
	Json                  bool     `yaml:"json"`
	DiskLimitSizeBytes    int64    `yaml:"disk_limit_size_bytes"`
	InsecureRegistries    []string `yaml:"insecure_registries"`
	GIDMappings           []string `yaml:"gid_mappings"`
	UIDMappings           []string `yaml:"uid_mappings"`
}

type Clean struct {
	IgnoreBaseImages []string `yaml:"ignore_images"`
	ThresholdBytes   int64    `yaml:"threshold_bytes"`
}

type Builder struct {
	config *Config
}

func NewBuilder(pathToYaml string) (*Builder, error) {
	config := Config{}

	if pathToYaml != "" {
		var err error
		config, err = load(pathToYaml)
		if err != nil {
			return nil, err
		}
	}

	b := &Builder{
		config: &config,
	}

	return b, nil
}

func (b *Builder) Build() (Config, error) {
	if b.config.Create.DiskLimitSizeBytes < 0 {
		return *b.config, errorspkg.New("invalid argument: disk limit cannot be negative")
	}

	if b.config.Clean.ThresholdBytes < 0 {
		return *b.config, errorspkg.New("invalid argument: clean threshold cannot be negative")
	}

	return *b.config, nil
}

func (b *Builder) WithInsecureRegistries(insecureRegistries []string) *Builder {
	if insecureRegistries == nil || len(insecureRegistries) == 0 {
		return b
	}

	b.config.Create.InsecureRegistries = insecureRegistries
	return b
}

func (b *Builder) WithIgnoreBaseImages(ignoreBaseImages []string) *Builder {
	if ignoreBaseImages == nil || len(ignoreBaseImages) == 0 {
		return b
	}

	b.config.Clean.IgnoreBaseImages = ignoreBaseImages
	return b
}

func (b *Builder) WithStorePath(storePath string, isSet bool) *Builder {
	if isSet || b.config.StorePath == "" {
		b.config.StorePath = storePath
	}

	return b
}

func (b *Builder) WithFSDriver(driver string, isSet bool) *Builder {
	if isSet || b.config.FSDriver == "" {
		b.config.FSDriver = driver
	}

	return b
}

func (b *Builder) WithDraxBin(draxBin string, isSet bool) *Builder {
	if isSet || b.config.DraxBin == "" {
		b.config.DraxBin = draxBin
	}
	return b
}

func (b *Builder) WithNewuidmapBin(newuidmapBin string, isSet bool) *Builder {
	if isSet || b.config.NewuidmapBin == "" {
		b.config.NewuidmapBin = newuidmapBin
	}
	return b
}

func (b *Builder) WithNewgidmapBin(newgidmapBin string, isSet bool) *Builder {
	if isSet || b.config.NewgidmapBin == "" {
		b.config.NewgidmapBin = newgidmapBin
	}
	return b
}

func (b *Builder) WithBtrfsBin(btrfsBin string, isSet bool) *Builder {
	if isSet || b.config.BtrfsBin == "" {
		b.config.BtrfsBin = btrfsBin
	}
	return b
}

func (b *Builder) WithMetronEndpoint(metronEndpoint string) *Builder {
	if metronEndpoint == "" {
		return b
	}

	b.config.MetronEndpoint = metronEndpoint
	return b
}

func (b *Builder) WithUIDMappings(uidMappings []string) *Builder {
	if uidMappings == nil || len(uidMappings) == 0 {
		return b
	}

	b.config.Create.UIDMappings = uidMappings
	return b
}

func (b *Builder) WithGIDMappings(gidMappings []string) *Builder {
	if gidMappings == nil || len(gidMappings) == 0 {
		return b
	}

	b.config.Create.GIDMappings = gidMappings
	return b
}

func (b *Builder) WithDiskLimitSizeBytes(limit int64, isSet bool) *Builder {
	if isSet {
		b.config.Create.DiskLimitSizeBytes = limit
	}
	return b
}

func (b *Builder) WithExcludeImageFromQuota(exclude, isSet bool) *Builder {
	if isSet {
		b.config.Create.ExcludeImageFromQuota = exclude
	}
	return b
}

func (b *Builder) WithCleanThresholdBytes(threshold int64, isSet bool) *Builder {
	if isSet {
		b.config.Clean.ThresholdBytes = threshold
	}
	return b
}

func (b *Builder) WithLogLevel(level string, isSet bool) *Builder {
	if isSet {
		b.config.LogLevel = level
	}
	return b
}

func (b *Builder) WithLogFile(filepath string) *Builder {
	if filepath != "" {
		b.config.LogFile = filepath
	}
	return b
}

func (b *Builder) WithClean(clean bool, noClean bool) *Builder {
	if clean {
		b.config.Create.WithClean = clean
	}

	if noClean {
		b.config.Create.WithClean = false
	}

	return b
}

func (b *Builder) WithJson(json bool, noJson bool) *Builder {
	if json {
		b.config.Create.Json = json
	}

	if noJson {
		b.config.Create.Json = false
	}

	return b
}

func load(configPath string) (Config, error) {
	configContent, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Config{}, errorspkg.Wrap(err, "invalid config path")
	}

	var config Config
	err = yaml.Unmarshal(configContent, &config)
	if err != nil {
		return Config{}, errorspkg.Wrap(err, "invalid config file")
	}

	return config, nil
}
