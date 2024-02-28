package guardiancmd

//nolint:staticcheck // SA5008 github.com/jesse-vdk/go-flag requires duplicate struct tags for 'choice'

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

	FormatUnixEpoch = "unix-epoch"
	FormatRFC3339   = "rfc3339"
)

type LagerFlag struct {
	LogLevel   string `long:"log-level" default:"info" choice:"debug" choice:"info" choice:"error" choice:"fatal" description:"Minimum level of logs to see."`
	TimeFormat string `long:"time-format" default:"unix-epoch" choice:"unix-epoch" choice:"rfc3339" description:"format of log timestamps."`
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

	var internalSink lager.Sink
	switch f.TimeFormat {
	case FormatRFC3339:
		internalSink = lager.NewPrettySink(os.Stdout, lager.DEBUG)
	default:
		internalSink = lager.NewWriterSink(os.Stdout, lager.DEBUG)
	}

	logger := lager.NewLogger(component)

	sink := lager.NewReconfigurableSink(internalSink, minLagerLogLevel)
	logger.RegisterSink(sink)

	return logger, sink
}
