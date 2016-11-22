package runner

func (r Runner) Stats(id string) error {
	_, err := r.RunSubcommand("stats", id)
	return err
}
