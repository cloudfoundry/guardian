package config

import (
	"io/ioutil"

	errorspkg "github.com/pkg/errors"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	StorePath      string `yaml:"store"`
	TardisBin      string `yaml:"tardis_bin"`
	NewuidmapBin   string `yaml:"newuidmap_bin"`
	NewgidmapBin   string `yaml:"newgidmap_bin"`
	MetronEndpoint string `yaml:"metron_endpoint"`
	LogLevel       string `yaml:"log_level"`
	LogFile        string `yaml:"log_file"`
	Create         Create `yaml:"create"`
	Clean          Clean  `yaml:"clean"`
	Init           Init   `yaml:"init"`
}

type Create struct {
	ExcludeImageFromQuota             bool     `yaml:"exclude_image_from_quota"`
	SkipLayerValidation               bool     `yaml:"skip_layer_validation"`
	WithClean                         bool     `yaml:"with_clean"`
	WithoutMount                      bool     `yaml:"without_mount"`
	DiskLimitSizeBytes                int64    `yaml:"disk_limit_size_bytes"`
	InsecureRegistries                []string `yaml:"insecure_registries"`
	RemoteLayerClientCertificatesPath string   `yaml:"remote_layer_client_certificates_path"`
}

type Clean struct {
	ThresholdBytes int64 `yaml:"threshold_bytes"`
}

type Init struct {
	StoreSizeBytes int64 `yaml:"store_size_bytes"`
	OwnerUser      string
	OwnerGroup     string
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

func (b *Builder) WithStorePath(storePath string, isSet bool) *Builder {
	if isSet || b.config.StorePath == "" {
		b.config.StorePath = storePath
	}

	return b
}

func (b *Builder) WithTardisBin(tardisBin string, isSet bool) *Builder {
	if isSet || b.config.TardisBin == "" {
		b.config.TardisBin = tardisBin
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

func (b *Builder) WithMetronEndpoint(metronEndpoint string) *Builder {
	if metronEndpoint == "" {
		return b
	}

	b.config.MetronEndpoint = metronEndpoint
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

func (b *Builder) WithSkipLayerValidation(skip, isSet bool) *Builder {
	if isSet {
		b.config.Create.SkipLayerValidation = skip
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
		b.config.Create.WithClean = true
	}

	if noClean {
		b.config.Create.WithClean = false
	}

	return b
}

func (b *Builder) WithMount(mount bool, noMount bool) *Builder {
	if mount {
		b.config.Create.WithoutMount = false
	}

	if noMount {
		b.config.Create.WithoutMount = true
	}

	return b
}

func (b *Builder) WithStoreSizeBytes(size int64) *Builder {
	b.config.Init.StoreSizeBytes = size
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
