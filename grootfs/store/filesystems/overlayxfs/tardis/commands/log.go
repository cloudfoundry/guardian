package commands
import (
	"io"
	"code.cloudfoundry.org/lager"
)
func createLoggingSink(writer io.Writer, logLevel lager.LogLevel, logFormat string) lager.Sink {
	if logFormat == "rfc3339" {
		return lager.NewPrettySink(writer, logLevel)
	}
	return lager.NewWriterSink(writer, logLevel)
}
