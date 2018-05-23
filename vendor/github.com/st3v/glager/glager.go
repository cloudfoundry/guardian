// Package glager provides Gomega matchers to verify lager logging.
//
// See https://github.com/onsi/gomega and https://code.cloudfoundry.org/lager
// for more information.
package glager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/types"
)

type logEntry lager.LogFormat

type logEntries []logEntry

type logEntryData lager.Data

type option func(*logEntry)

// TestLogger embedds an actual lager logger and implements gbytes.BufferProvider.
// This comes in handy when used with the HaveLogged matcher.
type TestLogger struct {
	lager.Logger
	buf *gbytes.Buffer
}

// NewLogger returns a new TestLogger that can be used with the HaveLogged matcher.
// The returned logger uses log level lager.DEBUG.
func NewLogger(component string) *TestLogger {
	buf := gbytes.NewBuffer()
	log := lager.NewLogger(component)
	log.RegisterSink(lager.NewWriterSink(buf, lager.DEBUG))
	return &TestLogger{log, buf}
}

// Buffer implements gbytes.BufferProvider.Buffer.
func (l *TestLogger) Buffer() *gbytes.Buffer {
	return l.buf
}

type logMatcher struct {
	actual   logEntries
	expected logEntries
}

// HaveLogged is an alias for ContainSequence. It checks if the specified entries
// appear inside the log in the right sequence. The entries do not have to be
// contiguous in the log, all that matters is the order and the properties of the
// entries.
//
// Log entries passed to this mtcher do not have to be complete, i.e. you do not
// have to specify all available properties but only the ones you are actually
// interested in. Unspecified properties are being ignored when matching the
// entries, i.e they can have arbitrary values.
//
// This matcher works best when used with a TestLogger.
//
// Example:
//   // instantiate glager.TestLogger inside your test and
//   logger := NewLogger("test")
//
//   // pass it to your code and use it in there to log stuff
//   myFunc(logger)
//
//   // verify logging inside your test, using the logger
//   // in this example we are interested in the log level, the message, and the
//   // data of the log entries
//   Expect(logger).To(HaveLogged(
// 	   Info(
// 		   Message("test.myFunc"),
// 		   Data("event", "start"),
// 	   ),
// 	   Info(
// 		   Message("test.myFunc"),
// 		   Data("event", "done"),
// 	   ),
//   ))
func HaveLogged(expectedSequence ...logEntry) types.GomegaMatcher {
	return ContainSequence(expectedSequence...)
}

// ContainSequence checks if the specified entries appear inside the log in the
// right order. The entries do not have to be contiguous inide the log, all that
// matters is their respective order.
//
// Log entries passed to this mtcher do not have to be complete, i.e. you do not
// have to specify all available properties but only the ones you are actually
// interested in. Unspecified properties are being ignored when matching the
// entries, i.e they can have arbitrary values.
//
// This matcher works best when used with a Buffer.
//
// Example:
//   // instantiate regular lager logger and register buffer as sink
//   log := gbytes.NewBuffer()
//   logger := lager.NewLogger("test")
//   logger.RegisterSink(lager.NewWriterSink(log, lager.DEBUG))
//
//   // pass it to your code and use it in there to log stuff
//   myFunc(logger)
//
//   // verify logging inside your test, using the log buffer
//   // in this example we are only interested in the data of the log entries
//   Expect(log).To(ContainSequence(
// 	   Info(
// 		   Data("event", "start"),
// 	   ),
// 	   Info(
// 		   Data("event", "done"),
// 	   ),
//   ))
func ContainSequence(expectedSequence ...logEntry) types.GomegaMatcher {
	return &logMatcher{
		expected: expectedSequence,
	}
}

// Info returns a log entry of type lager.INFO that can be used with the
// HaveLogged and ContainSequence matchers.
func Info(options ...option) logEntry {
	return Entry(lager.INFO, options...)
}

// Debug returns a log entry of type lager.DEBUG that can be used with the
// HaveLogged and ContainSequence matchers.
func Debug(options ...option) logEntry {
	return Entry(lager.DEBUG, options...)
}

// AnyErr can be used to check of arbitrary errors when matching Error entries.
var AnyErr error = nil

// Error returns a log entry of type lager.ERROR that can be used with the
// HaveLogged and ContainSequence matchers.
func Error(err error, options ...option) logEntry {
	if err != nil {
		options = append(options, Data("error", err.Error()))
	}

	return Entry(lager.ERROR, options...)
}

// Fatal returns a log entry of type lager.FATAL that can be used with the
// HaveLogged and ContainSequence matchers.
func Fatal(err error, options ...option) logEntry {
	if err != nil {
		options = append(options, Data("error", err.Error()))
	}

	return Entry(lager.FATAL, options...)
}

// Entry returns a log entry for the specified log level that can be used with
// the HaveLogged and ContainSequence matchers.
func Entry(logLevel lager.LogLevel, options ...option) logEntry {
	entry := logEntry(lager.LogFormat{
		LogLevel: logLevel,
		Data:     lager.Data{},
	})

	for _, option := range options {
		option(&entry)
	}

	return entry
}

// Message specifies a string that represent the message of a given log entry.
func Message(msg string) option {
	return func(e *logEntry) {
		e.Message = msg
	}
}

// Action is an alias for Message, lager uses the term action is used
// alternatively to message.
func Action(action string) option {
	return Message(action)
}

// Source specifies a string that indicates the log source. The source of a
// lager logger is usually specified at instantiation time. Source is sometimes
// also called component.
func Source(src string) option {
	return func(e *logEntry) {
		e.Source = src
	}
}

// Data specifies the data logged by a given log entry. Arguments are specified
// as an alternating sequence of keys (string) and values (interface{}).
func Data(kv ...interface{}) option {
	if len(kv)%2 == 1 {
		kv = append(kv, "")
	}

	return func(e *logEntry) {
		for i := 0; i < len(kv); i += 2 {
			key, ok := kv[i].(string)
			if !ok {
				err := fmt.Errorf("Invalid type for data key. Want string. Got %T:%v.", kv[i], kv[i])
				panic(err)
			}
			e.Data[key] = kv[i+1]
		}
	}
}

// ContentsProvider implements Contents function.
type ContentsProvider interface {
	// Contents returns a slice of bytes.
	Contents() []byte
}

// Match is doing the actual matching for a given log assertion.
func (lm *logMatcher) Match(actual interface{}) (success bool, err error) {
	var reader io.Reader

	switch x := actual.(type) {
	case gbytes.BufferProvider:
		reader = bytes.NewReader(x.Buffer().Contents())
	case ContentsProvider:
		reader = bytes.NewReader(x.Contents())
	case io.Reader:
		reader = x
	default:
		return false, fmt.Errorf("ContainSequence must be passed an io.Reader, glager.ContentsProvider, or gbytes.BufferProvider. Got:\n%s", format.Object(actual, 1))
	}

	decoder := json.NewDecoder(reader)

	lm.actual = logEntries{}

	for {
		var entry logEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			return false, err
		}
		lm.actual = append(lm.actual, entry)
	}

	actualEntries := lm.actual

	for _, expected := range lm.expected {
		i, found, err := actualEntries.indexOf(expected)
		if err != nil {
			return false, err
		}

		if !found {
			return false, nil
		}

		actualEntries = actualEntries[i+1:]
	}

	return true, nil
}

// FailureMessage constructs a message for failed assertions.
func (lm *logMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%s\nto contain log sequence \n\t%s",
		format.Object(lm.actual, 0),
		format.Object(lm.expected, 0),
	)
}

// NegatedFailureMessage constructs a message for failed negative assertions.
func (lm *logMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%s\nnot to contain log sequence \n\t%s",
		format.Object(lm.actual, 0),
		format.Object(lm.expected, 0),
	)
}

func (entry logEntry) logData() logEntryData {
	return logEntryData(entry.Data)
}

func (actual logEntry) contains(expected logEntry) (bool, error) {
	if expected.Source != "" && actual.Source != expected.Source {
		return false, nil
	}

	if expected.Message != "" && actual.Message != expected.Message {
		return false, nil
	}

	if actual.LogLevel != expected.LogLevel {
		return false, nil
	}

	containsData, err := actual.logData().contains(expected.logData())
	if err != nil {
		return false, err
	}

	return containsData, nil
}

func (actual logEntryData) contains(expected logEntryData) (bool, error) {
	for expectedKey, expectedVal := range expected {
		actualVal, found := actual[expectedKey]
		if !found {
			return false, nil
		}

		// this has been marshalled and unmarshalled before, no need to check err
		actualJSON, _ := json.Marshal(actualVal)

		expectedJSON, err := json.Marshal(expectedVal)
		if err != nil {
			return false, err
		}

		if string(actualJSON) != string(expectedJSON) {
			return false, nil
		}
	}
	return true, nil
}

func (entries logEntries) indexOf(entry logEntry) (int, bool, error) {
	for i, actual := range entries {
		containsEntry, err := actual.contains(entry)
		if err != nil {
			return 0, false, err
		}

		if containsEntry {
			return i, true, nil
		}
	}
	return 0, false, nil
}
