package runner

func (r *Runner) Clean() error {
	_, err := r.RunSubcommand("clean")
	return err
}
