package groot

type InsecureDockerRegistryErr struct {
	error
}

type ImageNotFoundErr struct {
	error
}

func NewInsecureDockerRegistryErr(err error) error {
	return &InsecureDockerRegistryErr{err}
}

func NewImageNotFoundErr(err error) error {
	return &ImageNotFoundErr{err}
}
