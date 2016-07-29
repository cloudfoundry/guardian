package group_runner

import (
	"fmt"
	"os"

	"github.com/tedsuo/ifrit"
)

type groupRunner struct {
	members []Member
}

type Member struct {
	Name string
	ifrit.Runner
}

type ExitEvent struct {
	Member Member
	Err    error
}

type ExitTrace []ExitEvent

func (trace ExitTrace) ToError() error {
	for _, exit := range trace {
		if exit.Err != nil {
			return trace
		}
	}
	return nil
}

func (m ExitTrace) Error() string {
	return fmt.Sprintf("")
}

func New(members []Member) ifrit.Runner {
	return &groupRunner{
		members: members,
	}
}

func (r *groupRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	processes := []ifrit.Process{}
	processChan := make(chan ifrit.Process)
	exitTrace := make(ExitTrace, 0, len(r.members))
	exitEvents := make(chan ExitEvent)
	shutdown := false

	go func() {
		for _, member := range r.members {
			process := ifrit.Background(member)
			go func(member Member) {
				err := <-process.Wait()
				exitEvents <- ExitEvent{
					Err:    err,
					Member: member,
				}
			}(member)
			processChan <- process
			<-process.Ready()
		}
		close(ready)
	}()

	for {
		select {
		case sig := <-signals:
			shutdown = true
			for _, process := range processes {
				process.Signal(sig)
			}

		case process := <-processChan:
			processes = append(processes, process)

		case exit := <-exitEvents:
			exitTrace = append(exitTrace, exit)

			if len(exitTrace) == len(processes) {
				return exitTrace.ToError()
			}

			if shutdown {
				break
			}

			shutdown = true
			for _, process := range processes {
				process.Signal(os.Interrupt)
			}
		}
	}
}
