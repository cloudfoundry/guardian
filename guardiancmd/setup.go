package guardiancmd

type SetupCommand struct{}

func (*SetupCommand) Execute(args []string) error {
	return nil
}
