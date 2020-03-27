package processwaiter

import "os"

func isProcessAlive(pid int) bool {
	_, err := os.FindProcess(pid)

	return err == nil
}
