package guardiancmd

import "code.cloudfoundry.org/guardian/gardener"

type CleanupCommand struct {
	*CommonCommand
}

func (cmd *CleanupCommand) Execute(args []string) error {
	log, _ := cmd.Logger.Logger("guardian-cleanup")

	wiring, err := cmd.createWiring(log)
	if err != nil {
		return err
	}
	wiring.Restorer = &gardener.NoopRestorer{}

	gardener := cmd.createGardener(wiring)

	if err := gardener.Cleanup(log); err != nil {
		return err
	}

	cmd.saveProperties(log, cmd.Containers.PropertiesPath, wiring.PropertiesManager)

	return nil
}
