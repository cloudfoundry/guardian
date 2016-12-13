package config

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

	return &Builder{
		config: &config,
	}, nil
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
