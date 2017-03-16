package runner

func (r Runner) InitStore() error {
	_, err := r.RunSubcommand("init-store")
	return err
}
