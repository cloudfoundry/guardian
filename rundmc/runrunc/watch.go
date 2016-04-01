package runrunc

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . EventsNotifier
type EventsNotifier interface {
	OnEvent(handle string, event string)
}

type OomWatcher struct {
	commandRunner command_runner.CommandRunner
	runc          RuncBinary
}

func NewOomWatcher(runner command_runner.CommandRunner, runc RuncBinary) *OomWatcher {
	return &OomWatcher{runner, runc}
}

type runcEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (r *OomWatcher) WatchEvents(log lager.Logger, handle string, eventsNotifier EventsNotifier) error {
	stdoutR, w := io.Pipe()

	cmd := r.runc.EventsCommand(handle)
	cmd.Stdout = w

	log = log.Session("watch", lager.Data{
		"handle": handle,
	})
	log.Info("watching")

	defer func() {
		w.Close()
		stdoutR.Close()
		log.Info("done")
	}()

	if err := r.commandRunner.Start(cmd); err != nil {
		log.Error("run-events", err)
		return fmt.Errorf("start: %s", err)
	}
	go r.commandRunner.Wait(cmd) // avoid zombie

	decoder := json.NewDecoder(stdoutR)
	for {
		log.Debug("wait-next-event")

		var event runcEvent
		err := decoder.Decode(&event)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode event: %s", err)
		}

		log.Debug("got-event", lager.Data{
			"type": event.Type,
		})
		if event.Type == "oom" {
			eventsNotifier.OnEvent(handle, "Out of memory")
		}
	}
}
