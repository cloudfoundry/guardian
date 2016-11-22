package runner

func (r Runner) Metrics(id string) error {
	_, err := r.RunSubcommand("metrics", id)
	return err
}
