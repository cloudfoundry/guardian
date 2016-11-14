package imageplugin

type Image struct {
	Config ImageConfig `json:"config,omitempty"`
}

type ImageConfig struct {
	Env []string `json:"Env,omitempty"`
}
