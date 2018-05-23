[![Build Status](https://travis-ci.org/st3v/glager.svg?branch=master)](https://travis-ci.org/st3v/glager)
[![Coverage Status](https://coveralls.io/repos/st3v/glager/badge.svg?branch=master)](https://coveralls.io/r/st3v/glager?branch=master)

glager
======

Package `glager` provides a set of [Gomega](https://github.com/onsi/gomega) matchers to verify  certain events have been logged using the [Lager](https://code.cloudfoundry.org/lager) format.

## Installation

```
go get github.com/st3v/glager
```

## Matchers

There are two matchers, `glager.HaveLogged` and `glager.ContainSequence`. While their behavior is identical, one might provide better test readability than the other depending on the test scenario. For example, `HaveLogged` works best when use with the included `glager.TestLogger`.

```go
logger := glager.NewLogger("test")

...

Expect(logger).To(HaveLogged(...))
```

`ContainSequence` on the other hand reads nice when used with `gbytes.Buffer` or `io.Reader`.

```go
log := gbytes.NewBuffer()
logger := lager.NewLogger("test")
logger.RegisterSink(lager.NewWriterSink(log, lager.DEBUG))

...

Expect(log).To(ContainSequence(...))
```

Both matchers verify that a certain sequence of log entries have been written using the lager logging format. Depending on the expected log level a log entry passed to the matcher can be specified using one the following methods.

```go
// Info specifies a log entry with level lager.INFO.
glager.Info(...)

// Debug specifies a log entry with level lager.DEBUG.
glager.Debug(...)

// Error specifies a log entry with level lager.ERROR.
glager.Error(...)

// Fatal specifies a log entry with level lager.FATAL.
glager.Fatal(...)
```

All of the above methods take a set of optional arguments used to specify the expected details of a given log entry. The available properties are:


```go
// Source specifies a string that indicates the log source. The source of a
// lager logger is usually specified at instantiation time. Source is sometimes
// also called component.
glager.Source("source")

// Message specifies a string that represent the message of a given log entry.
glager.Message("message")

// Action is an alias for Message, lager uses the term action is used
// alternatively to message.
glager.Action("action")

// Data specifies the data logged by a given log entry. Arguments are specified
// as an alternating sequence of keys (string) and values (interface{}).
glager.Data("key1", "value1", "key2", "value2", ...)

// AnyErr can be used to match an Error or Fatal log entry, without matching the
// actual error that has been logged.
glager.AnyErr
```

When passing a sequence of log entries to the matcher, you only have to include the entries you are actually interested in. They don't have to be contiguous entries in the log. All that matters is their properties and their respective order.

Similarly, you don't have to specify all possible properties of a given log entry but only the ones you actually care about. For example, if all that matters to you is that there were two subsequent calls to `logger.Info`, you could use:

```go
Expect(logger).To(HaveLogged(Info(), Info()))
```

If you care about the data that has been logged something like the following might work for you.

```go
Expect(logger).To(HaveLogged(
  Info(Data("some-string", "string-value")),
  Info(Data("some-integer", 12345)),
))
```

## Example Usage

See `example_test.go` for executable examples.

```go
import (
  "code.cloudfoundry.com/lager"
  . "github.com/onsi/gomega"
  . "github.com/st3v/glager"
)

...

// instantiate glager.TestLogger inside your test and
logger := NewLogger("test")

// pass logger to your code and use it in there to log stuff
func(logger *lager.Logger) {
  logger.Info("myFunc", lager.Data(map[string]interface{}{
    "event": "start",
  }))

  logger.Debug("myFunc", lager.Data(map[string]interface{}{
    "some": "stuff",
    "more": "stuff",
  }))

  logger.Error("myFunc",
    errors.New("some error"),
    lager.Data(map[string]interface{}{
      "details": "stuff",
    }),
  )

  logger.Info("myFunc", lager.Data(map[string]interface{}{
    "event": "done",
  }))
}()

...

// verify logging inside your test
Expect(logger).To(HaveLogged(
  Info(
    Message("test.myFunc"),
    Data("event", "start"),
  ),
  Debug(
    Data("more", "stuff"),
  ),
  Error(AnyErr),
))

...
```

## License
`glager` is licensed under the Apache License, Version 2.0. See [LICENSE](https://github.com/st3v/glager/blob/master/LICENSE) for details.
