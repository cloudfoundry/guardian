package runner

func (r Runner) DeleteStore() error {
	_, err := r.RunSubcommand("delete-store")
	return err
}
