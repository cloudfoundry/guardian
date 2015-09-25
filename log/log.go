package log

import (
	"io"

	"github.com/onsi/ginkgo"
	"github.com/pivotal-golang/lager"
)

var log = &chainLogger{lager.NewLogger("guardian-default")}

func init() {
	// unless overridden, use GinkgoWriter, which amounts to stdout if we're not in a test
	log.RegisterSink(lager.NewWriterSink(ginkgo.GinkgoWriter, lager.DEBUG))
}

func SetLogger(l lager.Logger) {
	log = &chainLogger{l}
}

func RegisterSink(sink lager.Sink) {
	log.RegisterSink(sink)
}

func Session(action string, data ...lager.Data) ChainLogger {
	return &chainLogger{log.Session(action, data...)}
}

func Debug(action string, data ...lager.Data) {
	log.Debug(action, data...)
}

func Info(action string, data ...lager.Data) {
	log.Info(action, data...)
}

func Error(action string, err error, data ...lager.Data) error {
	log.Error(action, err, data...)
	return err
}

func Fatal(action string, err error, data ...lager.Data) {
	log.Fatal(action, err, data...)
}

type ChainLogger interface {
	lager.Logger
	Data(data lager.Data) ChainLogger
	Start(session string, data ...lager.Data) ChainLogger
	Err(action string, err error, data ...lager.Data) error
	LogIfNotNil(action string, err error, data ...lager.Data)

	io.Writer
}

type chainLogger struct {
	lager.Logger
}

func (c *chainLogger) Start(action string, data ...lager.Data) ChainLogger {
	session := c.Session(action, data...)
	session.Info("starting")

	return &chainLogger{session}
}

func (c *chainLogger) Data(data lager.Data) ChainLogger {
	session := c.WithData(data)
	return &chainLogger{session}
}

func (c *chainLogger) Write(p []byte) (n int, err error) {
	c.Info("received-data", lager.Data{"data": string(p)})
	return len(p), nil
}

func (c *chainLogger) LogIfNotNil(action string, err error, data ...lager.Data) {
	if err != nil {
		c.Error(action, err, data...)
	}
}

func (c *chainLogger) Err(action string, err error, data ...lager.Data) error {
	c.Error(action, err, data...)
	return err
}
