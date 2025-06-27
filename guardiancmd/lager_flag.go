package guardiancmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager/v3"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

type LagerFlag struct {
	//lint:ignore SA5008 github.com/jesse-vdk/go-flag requires duplicate struct tags for 'choice'
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

	internalSink := lager.NewPrettySink(os.Stdout, lager.DEBUG)

	logger := lager.NewLogger(component)

	sink := lager.NewReconfigurableSink(internalSink, minLagerLogLevel)
	logger.RegisterSink(sink)

	return logger, sink
}
