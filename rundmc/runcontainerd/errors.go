package runcontainerd

import "fmt"

type ContainerNotFoundError struct {
	Handle string
}

func (ce ContainerNotFoundError) Error() string {
	return fmt.Sprintf("container %s not found", ce.Handle)
}

type TaskNotFoundError struct {
	Handle string
}

func (te TaskNotFoundError) Error() string {
	return fmt.Sprintf("task for container %s not found", te.Handle)
}

type ProcessNotFoundError struct {
	Handle string
	ID     string
}

func (pe ProcessNotFoundError) Error() string {
	return fmt.Sprintf("process %s not found in container %s", pe.ID, pe.Handle)
}
