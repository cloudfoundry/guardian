package throttle

import (
	"time"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Runnable
type Runnable interface {
	Run(logger lager.Logger) error
}

func NewPollingService(
	logger lager.Logger,
	runnable Runnable,
	ticker <-chan time.Time,
) *PollingService {
	return &PollingService{
		logger:      logger,
		runnable:    runnable,
		ticker:      ticker,
		stopChannel: make(chan struct{}),
	}
}

type PollingService struct {
	logger      lager.Logger
	runnable    Runnable
	ticker      <-chan time.Time
	stopChannel chan struct{}
}

func (s PollingService) run() {
	for {
		select {
		case <-s.stopChannel:
			return
		case <-s.ticker:
			if err := s.runnable.Run(s.logger); err != nil {
				s.logger.Error("failed-to-run-runnable", err)
			}
		}
	}
}

func (s PollingService) Start() {
	go s.run()
}

func (s PollingService) Stop() {
	s.stopChannel <- struct{}{}
}
