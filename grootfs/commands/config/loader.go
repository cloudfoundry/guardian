package config

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	BaseStorePath             string   `yaml:"store_path"`
	CleanThresholdBytes       uint64   `yaml:"clean_threshold_bytes"`
	DiskLimitSizeBytes        int64    `yaml:"disk_limit_size_bytes"`
	DraxBin                   string   `yaml:"drax_bin"`
	ExcludeBaseImageFromQuota bool     `yaml:"exclude_base_image_from_quota"`
	GIDMappings               []string `yaml:"gid_mappings"`
	UIDMappings               []string `yaml:"uid_mappings"`
	IgnoreBaseImages          []string `yaml:"ignore_base_images"`
	InsecureRegistries        []string `yaml:"insecure_registries"`
	MetronEndpoint            string   `yaml:"metron_endpoint"`
	UserBasedStorePath        string
}

func Load(configPath string) (Config, error) {
	configContent, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("invalid config path: %s", err)
	}

	var config Config
	err = yaml.Unmarshal(configContent, &config)
	if err != nil {
		return Config{}, fmt.Errorf("invalid config file: %s", err)
	}

	return config, nil
}
