package runrunc

import (
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/guardian/rundmc/event"
	"code.cloudfoundry.org/lager"
)

type OomWatcher struct {
	commandRunner commandrunner.CommandRunner
	runc          RuncBinary
	events        chan event.Event
}

func NewOomWatcher(runner commandrunner.CommandRunner, runc RuncBinary) *OomWatcher {
	return &OomWatcher{runner, runc, make(chan event.Event)}
}

type runcEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (r *OomWatcher) Events(log lager.Logger) (<-chan event.Event, error) {
	return r.events, nil
}

func (r *OomWatcher) WatchEvents(log lager.Logger, handle string) error {
	stdoutR, w := io.Pipe()

	cmd := r.runc.EventsCommand(handle)
	cmd.Stdout = w

	log = log.Session("watch", lager.Data{
		"handle": handle,
	})
	log.Info("watching")

	defer func() {
		stdoutR.Close()
		log.Info("done")
	}()

	if err := r.commandRunner.Start(cmd); err != nil {
		log.Error("run-events", err)
		return fmt.Errorf("start: %s", err)
	}

	go func() {
		defer w.Close()
		r.commandRunner.Wait(cmd) // avoid zombie
	}()

	decoder := json.NewDecoder(stdoutR)
	for {
		log.Debug("wait-next-event")

		var e runcEvent
		err := decoder.Decode(&e)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode event: %s", err)
		}

		log.Debug("got-event", lager.Data{
			"type": e.Type,
		})
		if e.Type == "oom" {
			r.events <- event.NewOOMEvent(handle)
		}
	}
}
