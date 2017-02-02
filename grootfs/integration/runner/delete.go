package runner

func (r Runner) Delete(id string) error {
	if id == "" {
		_, err := r.RunSubcommand("delete")
		return err
	}
	_, err := r.RunSubcommand("delete", id)
	return err
}
