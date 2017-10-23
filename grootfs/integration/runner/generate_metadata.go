package runner

func (r Runner) GenerateVolumeSizeMetadata() error {
	_, err := r.RunSubcommand("generate-volume-size-metadata")
	return err
}
