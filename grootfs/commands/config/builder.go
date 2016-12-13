package config

import "code.cloudfoundry.org/grootfs/commands/storepath"

type Builder struct {
	config *Config
}

func NewBuilder() *Builder {
	return &Builder{
		config: &Config{},
	}
}

func NewBuilderFromFile(pathToYaml string) (*Builder, error) {
	config, err := Load(pathToYaml)
	if err != nil {
		return nil, err
	}

	b := &Builder{
		config: &config,
	}

	return b.WithStorePath(config.BaseStorePath, ""), nil
}

func (b *Builder) Build() Config {
	return *b.config
}

func (b *Builder) WithInsecureRegistries(insecureRegistries []string) *Builder {
	if insecureRegistries == nil || len(insecureRegistries) == 0 {
		return b
	}

	b.config.InsecureRegistries = insecureRegistries
	return b
}

func (b *Builder) WithIgnoreBaseImages(ignoreBaseImages []string) *Builder {
	if ignoreBaseImages == nil || len(ignoreBaseImages) == 0 {
		return b
	}

	b.config.IgnoreBaseImages = ignoreBaseImages
	return b
}

func (b *Builder) WithStorePath(storePath, defaultStorePath string) *Builder {
	if b.config.BaseStorePath != "" && storePath == defaultStorePath {
		b.config.UserBasedStorePath = storepath.UserBased(b.config.BaseStorePath)
		return b
	}

	b.config.BaseStorePath = storePath
	b.config.UserBasedStorePath = storepath.UserBased(storePath)
	return b
}

func (b *Builder) WithDraxBin(draxBin, defaultDraxBin string) *Builder {
	if b.config.DraxBin != "" && draxBin == defaultDraxBin {
		return b
	}

	b.config.DraxBin = draxBin
	return b
}
