package guardiancmd

import (
	"fmt"
	"os"

	"github.com/pivotal-golang/lager"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

type LoggerProvider interface {
	Logger(string) (lager.Logger, *lager.ReconfigurableSink)
}

type LagerFlag struct {
	LogLevel string `long:"log-level" default:"info" choice:"debug" choice:"info" choice:"error" choice:"fatal" description:"Minimum level of logs to see."`
}

func (f LagerFlag) Logger(component string) (lager.Logger, *lager.ReconfigurableSink) {
	var minLagerLogLevel lager.LogLevel
	switch f.LogLevel {
	case LogLevelDebug:
		minLagerLogLevel = lager.DEBUG
	case LogLevelInfo:
		minLagerLogLevel = lager.INFO
	case LogLevelError:
		minLagerLogLevel = lager.ERROR
	case LogLevelFatal:
		minLagerLogLevel = lager.FATAL
	default:
		panic(fmt.Sprintf("unknown log level: %s", f.LogLevel))
	}

	logger := lager.NewLogger(component)

	sink := lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, lager.DEBUG), minLagerLogLevel)
	logger.RegisterSink(sink)

	return logger, sink
}
