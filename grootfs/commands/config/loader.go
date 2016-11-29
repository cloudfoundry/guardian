package config

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	InsecureRegistries []string `yaml:"insecure_registries"`
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
