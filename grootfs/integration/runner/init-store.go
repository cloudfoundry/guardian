package runner

func (r Runner) InitStore() (string, error) {
	return r.RunSubcommand("init-store")
}
