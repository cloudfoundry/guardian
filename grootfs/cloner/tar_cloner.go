package cloner

import (
	"fmt"
	"os"
	"os/exec"

	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type TarCloner struct {
}

func NewTarCloner() *TarCloner {
	return &TarCloner{}
}

func (c *TarCloner) Clone(logger lager.Logger, spec grootpkg.CloneSpec) error {
	if _, err := os.Stat(spec.FromDir); err != nil {
		return fmt.Errorf("image path `%s` was not found: %s", spec.FromDir, err)
	}

	tarCmd := exec.Command("tar", "-cp", "-C", spec.FromDir, ".")
	tarCmd.Stderr = os.Stderr

	_ = os.Mkdir(spec.ToDir, 0755)

	untarCmd := exec.Command("tar", "-xp", "-C", spec.ToDir)
	untarCmd.Stdin, _ = tarCmd.StdoutPipe()
	untarCmd.Stdout = os.Stderr
	untarCmd.Stderr = os.Stderr
	_ = untarCmd.Start()

	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("reading from `%s`: %s", spec.FromDir, err)
	}

	if err := untarCmd.Wait(); err != nil {
		return fmt.Errorf("writing to `%s`: %s", spec.ToDir, err)
	}

	return nil
}
