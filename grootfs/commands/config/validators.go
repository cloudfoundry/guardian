package config

import "os/exec"

func ValidateBinary(path string) error {
	_, err := exec.LookPath(path)
	return err
}
