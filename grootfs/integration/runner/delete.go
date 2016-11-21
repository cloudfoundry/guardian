package runner

func (r Runner) Delete(id string) error {
	_, err := r.RunSubcommand("delete", id)
	return err
}
