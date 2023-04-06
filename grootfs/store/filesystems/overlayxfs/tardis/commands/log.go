package commands
import (
	"io"
	"code.cloudfoundry.org/lager/v3"
)
func createLoggingSink(writer io.Writer, logLevel lager.LogLevel, logFormat string) lager.Sink {
	if logFormat == "rfc3339" {
		return lager.NewPrettySink(writer, logLevel)
	}
	return lager.NewWriterSink(writer, logLevel)
}
