# Logrus <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/> [![Build Status](https://github.com/bnulwh/logrus/workflows/CI/badge.svg)](https://github.com/bnulwh/logrus/actions?query=workflow%3ACI) [![Go Reference](https://pkg.go.dev/badge/github.com/bnulwh/logrus.svg)](https://pkg.go.dev/github.com/bnulwh/logrus)

Logrus is a structured logger for Go (golang), completely API compatible with the standard library logger.

> **This project is a maintained fork of Logrus.** On top of the full upstream feature set, it adds **local file-system logging**, a **pure-stdlib rotating file writer**, and a **simple log formatter (SimpleFormatter)** that are ready to use out of the box — well suited for medium and large applications.

## Features

- Fully compatible with the standard library `log`; drop-in replacement for `log` imports
- Seven log levels: `Trace`, `Debug`, `Info`, `Warning`, `Error`, `Fatal`, `Panic`
- Structured logging via fields (`WithField` / `WithFields`) — no more long, unparseable messages
- Automatic color output when attached to a TTY; logfmt-compatible output otherwise
- Extensible via custom Formatters and Hooks
- ✅ **Local file-system logging**: `ConfigLocalFileSystemLogger` writes per-level rotating files with a single call
- ✅ **Pure-stdlib rotating writer**: `RotatingFileWriter` rotates by time/size and cleans up old files automatically (no third-party dependencies)
- ✅ **SimpleFormatter**: outputs `[time] [level] [file:line:func()] : message`
- ✅ **Global level helpers**: `IsTraceEnabled`/`IsDebugEnabled`/`IsInfoEnabled` and friends
- ✅ Configurable log retention: `SetMaxAge` / `GetMaxAge`

## Installation

```bash
go get github.com/bnulwh/logrus
```

> Note: use the **lowercase** import path `github.com/bnulwh/logrus` (Go import paths are case-sensitive).

## Quick Start

The simplest way to use Logrus is simply the package-level exported logger:

```go
package main

import (
  log "github.com/bnulwh/logrus"
)

func main() {
  log.WithFields(log.Fields{
    "animal": "walrus",
  }).Info("A walrus appears")
}
```

Since it is completely API-compatible with the stdlib logger, you can replace all your `log` imports with:

```go
import log "github.com/bnulwh/logrus"
```

and you'll now have the flexibility of Logrus. You can customize it all you want:

```go
package main

import (
  "os"
  log "github.com/bnulwh/logrus"
)

func init() {
  // Log as JSON instead of the default ASCII formatter.
  log.SetFormatter(&log.JSONFormatter{})

  // Output to stdout instead of the default stderr
  // Can be any io.Writer, see the file example below
  log.SetOutput(os.Stdout)

  // Only log the warning severity or above.
  log.SetLevel(log.WarnLevel)
}

func main() {
  log.WithFields(log.Fields{
    "animal": "walrus",
    "size":   10,
  }).Info("A group of walrus emerges from the ocean")

  log.WithFields(log.Fields{
    "omg":    true,
    "number": 122,
  }).Warn("The group's number increased tremendously!")

  log.WithFields(log.Fields{
    "omg":    true,
    "number": 100,
  }).Fatal("The ice breaks!")

  // A common pattern is to re-use fields between logging statements by re-using
  // the logrus.Entry returned from WithFields()
  contextLogger := log.WithFields(log.Fields{
    "common": "this is a common field",
    "other":  "I also should be logged always",
  })

  contextLogger.Info("I'll be logged with common and other field")
  contextLogger.Info("Me too")
}
```

For more advanced usage such as logging to multiple locations from the same application, you can also create an instance of the `logrus` Logger:

```go
package main

import (
  "os"
  "github.com/bnulwh/logrus"
)

// Create a new instance of the logger. You can have any number of instances.
var log = logrus.New()

func main() {
  // The API for setting attributes is a little different than the package level
  // exported logger. See GoDoc.
  log.Out = os.Stdout

  // You could set this to any `io.Writer` such as a file
  // file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
  // if err == nil {
  //  log.Out = file
  // } else {
  //  log.Info("Failed to log to file, using default stderr")
  // }

  log.WithFields(logrus.Fields{
    "animal": "walrus",
    "size":   10,
  }).Info("A group of walrus emerges from the ocean")
}
```

## 🆕 Local File-System Logging (fork addition)

Call `ConfigLocalFileSystemLogger` and logs are written per-level to rotating files, plus a combined log (common), out of the box:

```go
package main

import (
  "time"

  log "github.com/bnulwh/logrus"
)

func main() {
  // log directory + log file name (without extension)
  log.ConfigLocalFileSystemLogger("/var/log/myapp", "app.log")
  // Optional: set retention period (default 7 days)
  log.SetMaxAge(7 * 24 * time.Hour)

  log.Info("an info log")
  log.Error("an error log")
}
```

File layout (rotated hourly; the latest file is kept symlinked):

```
/var/log/myapp/
├── app.log.log            # combined log (all levels) → symlink to newest rotated file
├── app.log.debug.log      # debug-level log
├── app.log.info.log       # info-level log
├── app.log.warn.log       # warn-level log
└── app.log.error.log      # error/fatal/panic-level log
```

- `debug`/`info`/`warn` entries are written to both their level file and the combined file
- `error`/`fatal`/`panic` entries are written to both the error file and the combined file
- Rotation period: hourly; retention: configurable via `SetMaxAge` (default 7 days), expired files are cleaned up automatically
- Pure-stdlib implementation, no third-party dependencies

### Level checks

```go
if log.IsDebugEnabled() {
  log.Debug("debug mode is on")
}
// Also available: IsTraceEnabled / IsInfoEnabled / IsWarnEnabled / IsErrorEnabled / IsFatalEnabled / IsPanicEnabled
```

## 🆕 SimpleFormatter (fork addition)

A compact text format with optional color output and caller location:

```
[2024-01-01 12:00:00.123] [   info] : a plain info message
[2024-01-01 12:00:01.456] [   info] [ main.go : 42 : main() ] : message with caller
```

```go
log.SetFormatter(&log.SimpleFormatter{
  Colored: true, // enable colors
})
// enable caller info (file:line:func)
log.SetReportCaller(true)
```

## Level logging

Logrus has seven logging levels: Trace, Debug, Info, Warning, Error, Fatal and Panic.

```go
log.Trace("Something very low level.")
log.Debug("Useful debugging information.")
log.Info("Something noteworthy happened!")
log.Warn("You should probably take a look at this.")
log.Error("Something failed but I'm not quitting.")
// Calls os.Exit(1) after logging
log.Fatal("Bye.")
// Calls panic() after logging
log.Panic("I'm bailing.")
```

You can set the logging level on a `Logger`, then it will only log entries with that severity or anything above it:

```go
// Will log anything that is info or above (warn, error, fatal, panic). Default.
log.SetLevel(log.InfoLevel)
```

It may be useful to set `log.Level = logrus.DebugLevel` in a debug or verbose environment if your application has that.

## Fields

Logrus encourages careful, structured logging through logging fields instead of long, unparseable error messages. For example, instead of: `log.Fatalf("Failed to send event %s to topic %s with key %d")`, you should log the much more discoverable:

```go
log.WithFields(log.Fields{
  "event": event,
  "topic": topic,
  "key":   key,
}).Fatal("Failed to send event")
```

This API forces you to think about logging in a way that produces much more useful logging messages. The `WithFields` call is optional.

### Default Fields

Often it's helpful to have fields _always_ attached to log statements in an application or parts of one, e.g. always logging `request_id` and `user_ip` in the context of a request:

```go
requestLogger := log.WithFields(log.Fields{"request_id": request_id, "user_ip": user_ip})
requestLogger.Info("something happened on that request") // will log request_id and user_ip
requestLogger.Warn("something not great happened")
```

### Auto-added fields

Besides the fields added with `WithField` or `WithFields`, some fields are automatically added to all logging events:

1. `time` — the timestamp when the entry was created
2. `msg` — the logging message
3. `level` — the logging level

## Hooks

You can add hooks for logging levels — for example, to send errors to an exception tracking service on `Error`, `Fatal` and `Panic`, info to StatsD, or log to multiple places simultaneously (e.g. syslog).

The repo ships with [built-in hooks](hooks/) (`test`, `writer`, `syslog`), and you can write your own and add them in `init`:

```go
import (
  log "github.com/bnulwh/logrus"
  logrus_syslog "github.com/bnulwh/logrus/hooks/syslog"
  "log/syslog"
)

func init() {
  hook, err := logrus_syslog.NewSyslogHook("udp", "localhost:514", syslog.LOG_INFO, "")
  if err != nil {
    log.Error("Unable to connect to local syslog daemon")
  } else {
    log.AddHook(hook)
  }
}
```

> Note: the syslog hook also supports connecting to a local syslog (e.g. `/dev/log`, `/var/run/syslog`, `/var/run/log`). See the [syslog hook README](hooks/syslog/README.md) for details.

## Formatters

The built-in logging formatters are:

- `logrus.TextFormatter` — logs the event in colors if stdout is a tty, otherwise without colors.
  - To force colored output when there is no TTY, set `ForceColors: true`; to force no colored output even if there is a TTY, set `DisableColors: true`.
  - When colors are enabled, levels are truncated to 4 characters by default. Set `DisableLevelTruncation: true` to disable truncation.
  - When outputting to a TTY, setting `PadLevelText: true` adds padding to the level text for easy visual scanning.
- `logrus.JSONFormatter` — logs fields as JSON, easy to parse with Logstash / Splunk.
- `logrus.SimpleFormatter` — (fork addition) compact text format, see above.

You can define your own formatter by implementing the `Formatter` interface:

```go
type MyJSONFormatter struct {
}

log.SetFormatter(new(MyJSONFormatter))

func (f *MyJSONFormatter) Format(entry *Entry) ([]byte, error) {
  // Note this doesn't include Time, Level and Message which are available on
  // the Entry. Consult `godoc` on information about those fields.
  serialized, err := json.Marshal(entry.Data)
  if err != nil {
    return nil, fmt.Errorf("Failed to marshal fields to JSON, %w", err)
  }
  return append(serialized, '\n'), nil
}
```

## Logging Method Name

If you wish to add the calling method as a field, instruct the logger via:

```go
log.SetReportCaller(true)
```

This adds the caller as the `method` field.

> Note: this does add measurable overhead — between 20 and 40% in tests with Go 1.6/1.7. Validate in your environment via benchmarks:
> ```
> go test -bench=.*CallerTracing
> ```

## Logger as an `io.Writer`

Logrus can be transformed into an `io.Writer`. That writer is the end of an `io.Pipe` and it is your responsibility to close it:

```go
w := logger.Writer()
defer w.Close()

srv := http.Server{
  // create a stdlib log.Logger that writes to logrus.Logger
  ErrorLog: log.New(w, "", 0),
}
```

Each line written to that writer will be printed the usual way, using formatters and hooks. The level for those entries is `info`.

This means that we can override the standard library logger easily:

```go
logger := logrus.New()
logger.Formatter = &logrus.JSONFormatter{}

// Use logrus for standard log output
// Note that `log` here references stdlib's log
log.SetOutput(logger.Writer())
```

## Testing

Logrus has a built in facility for asserting the presence of log messages, implemented through the `test` hook:

- decorators for an existing logger (`test.NewLocal` and `test.NewGlobal`)
- a test logger (`test.NewNullLogger`) that just records log messages (and does not output any):

```go
import (
  "testing"

  "github.com/bnulwh/logrus"
  "github.com/bnulwh/logrus/hooks/test"
  "github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
  logger, hook := test.NewNullLogger()
  logger.Error("Hello error")

  assert.Equal(t, 1, len(hook.Entries))
  assert.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
  assert.Equal(t, "Hello error", hook.LastEntry().Message)

  hook.Reset()
  assert.Nil(t, hook.LastEntry())
}
```

## Fatal handlers

Logrus can register one or more functions that will be called when any `fatal` level message is logged. The registered handlers will be executed before logrus performs an `os.Exit(1)`. This may be helpful if callers need to gracefully shut down. Unlike a `panic("Something went wrong...")` call which can be intercepted with a deferred `recover`, a call to `os.Exit(1)` cannot be intercepted.

```go
handler := func() {
  // gracefully shutdown something...
}
logrus.RegisterExitHandler(handler)
```

## Thread safety

By default, Logger is protected by a mutex for concurrent writes. The mutex is held when calling hooks and writing logs. If you are sure such locking is not needed, you can call `logger.SetNoLock()` to disable the locking.

Situation when locking is not needed includes:

- You have no hooks registered, or hook calls are already thread-safe
- Writing to `logger.Out` is already thread-safe, for example:
  1. `logger.Out` is protected by locks
  2. `logger.Out` is an `os.File` handler opened with the `O_APPEND` flag, and every write is smaller than 4k (this allows multi-thread/multi-process writing)

## License

This project is released under the [MIT license](LICENSE), same as upstream Logrus.
