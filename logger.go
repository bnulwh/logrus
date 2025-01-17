package logrus

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// LogFunction For big messages, it can be more efficient to pass a function
// and only call it if the log level is actually enables rather than
// generating the log message and then checking if the level is enabled
type LogFunction func() []interface{}

type Logger struct {
	// The logs are `io.Copy`'d to this in a mutex. It's common to set this to a
	// file, or leave it default which is `os.Stderr`. You can also set this to
	// something more adventurous, such as logging to Kafka.
	Out io.Writer
	// Hooks for the logger instance. These allow firing events based on logging
	// levels and log entries. For example, to send errors to an error tracking
	// service, log to StatsD or dump the core on fatal errors.
	Hooks LevelHooks
	// All log entries pass through the formatter before logged to Out. The
	// included formatters are `TextFormatter` and `JSONFormatter` for which
	// TextFormatter is the default. In development (when a TTY is attached) it
	// logs with colors, but to a file it wouldn't. You can easily implement your
	// own that implements the `Formatter` interface, see the `README` or included
	// formatters for examples.
	Formatter Formatter

	// Flag for whether to log caller info (off by default)
	ReportCaller bool

	// The logging level the logger should log at. This is typically (and defaults
	// to) `logrus.Info`, which allows Info(), Warn(), Error() and Fatal() to be
	// logged.
	ConsoleLevel Level
	HookLevel    Level
	MaxAge       time.Duration
	// Used to sync writing to the log. Locking is enabled by Default
	mu MutexWrap
	// Reusable empty entry
	entryPool sync.Pool
	// Function to exit the application, defaults to `os.Exit()`
	ExitFunc exitFunc
	// The buffer pool used to format the log. If it is nil, the default global
	// buffer pool will be used.
	BufferPool BufferPool
}

type exitFunc func(int)

type MutexWrap struct {
	lock     sync.Mutex
	disabled bool
}

func (mw *MutexWrap) Lock() {
	if !mw.disabled {
		mw.lock.Lock()
	}
}

func (mw *MutexWrap) Unlock() {
	if !mw.disabled {
		mw.lock.Unlock()
	}
}

func (mw *MutexWrap) Disable() {
	mw.disabled = true
}

// Creates a new logger. Configuration should be set by changing `Formatter`,
// `Out` and `Hooks` directly on the default logger instance. You can also just
// instantiate your own:
//
//    var log = &logrus.Logger{
//      Out: os.Stderr,
//      Formatter: new(logrus.TextFormatter),
//      Hooks: make(logrus.LevelHooks),
//      Level: logrus.DebugLevel,
//    }
//
// It's recommended to make this a global instance called `log`.
func New() *Logger {
	return &Logger{
		Out:          os.Stdout,
		Formatter:    &SimpleFormatter{Colored: true},
		Hooks:        make(LevelHooks),
		ConsoleLevel: DebugLevel,
		HookLevel:    DebugLevel,
		ExitFunc:     os.Exit,
		ReportCaller: true,
		MaxAge:       time.Hour * 24 * 7,
	}
}

func (logger *Logger) newEntry() *Entry {
	entry, ok := logger.entryPool.Get().(*Entry)
	if ok {
		entry.ConsoleLevel = logger.ConsoleLevel
		entry.HookLevel = logger.HookLevel
		return entry
	}
	return NewEntry(logger)
}

func (logger *Logger) releaseEntry(entry *Entry) {
	entry.Data = map[string]interface{}{}
	logger.entryPool.Put(entry)
}

// WithField allocates a new entry and adds a field to it.
// Debug, Print, Info, Warn, Error, Fatal or Panic must be then applied to
// this new returned entry.
// If you want multiple fields, use `WithFields`.
func (logger *Logger) WithField(key string, value interface{}) *Entry {
	entry := logger.newEntry()
	defer logger.releaseEntry(entry)
	return entry.WithField(key, value)
}

// Adds a struct of fields to the log entry. All it does is call `WithField` for
// each `Field`.
func (logger *Logger) WithFields(fields Fields) *Entry {
	entry := logger.newEntry()
	defer logger.releaseEntry(entry)
	return entry.WithFields(fields)
}

// Add an error as single field to the log entry.  All it does is call
// `WithError` for the given `error`.
func (logger *Logger) WithError(err error) *Entry {
	entry := logger.newEntry()
	defer logger.releaseEntry(entry)
	return entry.WithError(err)
}

// Add a context to the log entry.
func (logger *Logger) WithContext(ctx context.Context) *Entry {
	entry := logger.newEntry()
	defer logger.releaseEntry(entry)
	return entry.WithContext(ctx)
}

// Overrides the time of the log entry.
func (logger *Logger) WithTime(t time.Time) *Entry {
	entry := logger.newEntry()
	defer logger.releaseEntry(entry)
	return entry.WithTime(t)
}

func (logger *Logger) Logf(level Level, format string, args ...interface{}) {
	if logger.IsLevelEnabled(level) {
		entry := logger.newEntry()
		entry.Logf(level, format, args...)
		logger.releaseEntry(entry)
	}
}
func (logger *Logger) Log(level Level, args ...interface{}) {
	if logger.IsLevelEnabled(level) {
		entry := logger.newEntry()
		entry.Log(level, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) LogFn(level Level, fn LogFunction) {
	if logger.IsLevelEnabled(level) {
		entry := logger.newEntry()
		entry.Log(level, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Tracef(format string, args ...interface{}) {
	//logger.Logf(TraceLevel, format, args...)
	if logger.IsLevelEnabled(TraceLevel) {
		entry := logger.newEntry()
		entry.Logf(TraceLevel, format, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Debugf(format string, args ...interface{}) {
	//logger.Logf(DebugLevel, format, args...)
	if logger.IsLevelEnabled(DebugLevel) {
		entry := logger.newEntry()
		entry.Logf(DebugLevel, format, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Infof(format string, args ...interface{}) {
	//logger.Logf(InfoLevel, format, args...)
	if logger.IsLevelEnabled(InfoLevel) {
		entry := logger.newEntry()
		entry.Logf(InfoLevel, format, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Printf(format string, args ...interface{}) {
	entry := logger.newEntry()
	entry.Printf(format, args...)
	logger.releaseEntry(entry)
}

func (logger *Logger) Warnf(format string, args ...interface{}) {
	//logger.Logf(WarnLevel, format, args...)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Logf(WarnLevel, format, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Warningf(format string, args ...interface{}) {
	//logger.Logf(WarnLevel, format, args...)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Logf(WarnLevel, format, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Errorf(format string, args ...interface{}) {
	//logger.Logf(ErrorLevel, format, args...)
	if logger.IsLevelEnabled(ErrorLevel) {
		entry := logger.newEntry()
		entry.Logf(ErrorLevel, format, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Fatalf(format string, args ...interface{}) {
	//logger.Logf(FatalLevel, format, args...)
	if logger.IsLevelEnabled(FatalLevel) {
		entry := logger.newEntry()
		entry.Logf(FatalLevel, format, args...)
		logger.releaseEntry(entry)
	}
	logger.Exit(1)
}

func (logger *Logger) Panicf(format string, args ...interface{}) {
	//logger.Logf(PanicLevel, format, args...)
	if logger.IsLevelEnabled(PanicLevel) {
		entry := logger.newEntry()
		entry.Logf(PanicLevel, format, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Trace(args ...interface{}) {
	//logger.Log(TraceLevel, args...)
	if logger.IsLevelEnabled(TraceLevel) {
		entry := logger.newEntry()
		entry.Log(TraceLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Debug(args ...interface{}) {
	//logger.Log(DebugLevel, args...)
	if logger.IsLevelEnabled(DebugLevel) {
		entry := logger.newEntry()
		entry.Log(DebugLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Info(args ...interface{}) {
	//logger.Log(InfoLevel, args...)
	if logger.IsLevelEnabled(InfoLevel) {
		entry := logger.newEntry()
		entry.Log(InfoLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Print(args ...interface{}) {
	entry := logger.newEntry()
	entry.Print(args...)
	logger.releaseEntry(entry)
}

func (logger *Logger) Warn(args ...interface{}) {
	//logger.Log(WarnLevel, args...)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Log(WarnLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Warning(args ...interface{}) {
	//logger.Warn(args...)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Log(WarnLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Error(args ...interface{}) {
	//logger.Log(ErrorLevel, args...)
	if logger.IsLevelEnabled(ErrorLevel) {
		entry := logger.newEntry()
		entry.Log(ErrorLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Fatal(args ...interface{}) {
	//logger.Log(FatalLevel, args...)
	if logger.IsLevelEnabled(FatalLevel) {
		entry := logger.newEntry()
		entry.Log(FatalLevel, args...)
		logger.releaseEntry(entry)
	}
	logger.Exit(1)
}

func (logger *Logger) Panic(args ...interface{}) {
	//logger.Log(PanicLevel, args...)
	if logger.IsLevelEnabled(PanicLevel) {
		entry := logger.newEntry()
		entry.Log(PanicLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) TraceFn(fn LogFunction) {
	//logger.LogFn(TraceLevel, fn)
	if logger.IsLevelEnabled(TraceLevel) {
		entry := logger.newEntry()
		entry.Log(TraceLevel, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) DebugFn(fn LogFunction) {
	//logger.LogFn(DebugLevel, fn)
	if logger.IsLevelEnabled(DebugLevel) {
		entry := logger.newEntry()
		entry.Log(DebugLevel, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) InfoFn(fn LogFunction) {
	//logger.LogFn(InfoLevel, fn)
	if logger.IsLevelEnabled(InfoLevel) {
		entry := logger.newEntry()
		entry.Log(InfoLevel, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) PrintFn(fn LogFunction) {
	entry := logger.newEntry()
	entry.Print(fn()...)
	logger.releaseEntry(entry)
}

func (logger *Logger) WarnFn(fn LogFunction) {
	//logger.LogFn(WarnLevel, fn)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Log(WarnLevel, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) WarningFn(fn LogFunction) {
	//logger.WarnFn(fn)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Log(WarnLevel, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) ErrorFn(fn LogFunction) {
	//logger.LogFn(ErrorLevel, fn)
	if logger.IsLevelEnabled(ErrorLevel) {
		entry := logger.newEntry()
		entry.Log(ErrorLevel, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) FatalFn(fn LogFunction) {
	//logger.LogFn(FatalLevel, fn)
	if logger.IsLevelEnabled(FatalLevel) {
		entry := logger.newEntry()
		entry.Log(FatalLevel, fn()...)
		logger.releaseEntry(entry)
	}
	logger.Exit(1)
}

func (logger *Logger) PanicFn(fn LogFunction) {
	//logger.LogFn(PanicLevel, fn)
	if logger.IsLevelEnabled(PanicLevel) {
		entry := logger.newEntry()
		entry.Log(PanicLevel, fn()...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Logln(level Level, args ...interface{}) {
	if logger.IsLevelEnabled(level) {
		entry := logger.newEntry()
		entry.Logln(level, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Traceln(args ...interface{}) {
	//logger.Logln(TraceLevel, args...)
	if logger.IsLevelEnabled(TraceLevel) {
		entry := logger.newEntry()
		entry.Logln(TraceLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Debugln(args ...interface{}) {
	//logger.Logln(DebugLevel, args...)
	if logger.IsLevelEnabled(DebugLevel) {
		entry := logger.newEntry()
		entry.Logln(DebugLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Infoln(args ...interface{}) {
	//logger.Logln(InfoLevel, args...)
	if logger.IsLevelEnabled(InfoLevel) {
		entry := logger.newEntry()
		entry.Logln(InfoLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Println(args ...interface{}) {
	entry := logger.newEntry()
	entry.Println(args...)
	logger.releaseEntry(entry)
}

func (logger *Logger) Warnln(args ...interface{}) {
	//logger.Logln(WarnLevel, args...)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Logln(WarnLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Warningln(args ...interface{}) {
	//logger.Warnln(args...)
	if logger.IsLevelEnabled(WarnLevel) {
		entry := logger.newEntry()
		entry.Logln(WarnLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Errorln(args ...interface{}) {
	//logger.Logln(ErrorLevel, args...)
	if logger.IsLevelEnabled(ErrorLevel) {
		entry := logger.newEntry()
		entry.Logln(ErrorLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Fatalln(args ...interface{}) {
	//logger.Logln(FatalLevel, args...)
	if logger.IsLevelEnabled(FatalLevel) {
		entry := logger.newEntry()
		entry.Logln(FatalLevel, args...)
		logger.releaseEntry(entry)
	}
	logger.Exit(1)
}

func (logger *Logger) Panicln(args ...interface{}) {
	//logger.Logln(PanicLevel, args...)
	if logger.IsLevelEnabled(PanicLevel) {
		entry := logger.newEntry()
		entry.Logln(PanicLevel, args...)
		logger.releaseEntry(entry)
	}
}

func (logger *Logger) Exit(code int) {
	runHandlers()
	if logger.ExitFunc == nil {
		logger.ExitFunc = os.Exit
	}
	logger.ExitFunc(code)
}

//When file is opened with appending mode, it's safe to
//write concurrently to a file (within 4k message on Linux).
//In these cases user can choose to disable the lock.
func (logger *Logger) SetNoLock() {
	logger.mu.Disable()
}

func (logger *Logger) consoleLevel() Level {
	return Level(atomic.LoadUint32((*uint32)(&logger.ConsoleLevel)))
}
func (logger *Logger) hookLevel() Level {
	return Level(atomic.LoadUint32((*uint32)(&logger.HookLevel)))
}

// SetLevel sets the logger level.
func (logger *Logger) SetLevel(levels ...Level) {
	if len(levels) > 0 {
		atomic.StoreUint32((*uint32)(&logger.ConsoleLevel), uint32(levels[0]))
		atomic.StoreUint32((*uint32)(&logger.HookLevel), uint32(levels[0]))
	}
	if len(levels) > 1 {
		atomic.StoreUint32((*uint32)(&logger.HookLevel), uint32(levels[1]))
	}
}

// GetLevel returns the logger level.
func (logger *Logger) GetLevel() Level {
	return logger.consoleLevel()
}

func (logger *Logger) GetMaxAge() time.Duration {
	return logger.MaxAge
}

// AddHook adds a hook to the logger hooks.
func (logger *Logger) AddHook(hook Hook) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Hooks.Add(hook)
}

// IsLevelEnabled checks if the log level of the logger is greater than the level param
func (logger *Logger) IsLevelEnabled(level Level) bool {
	return logger.consoleLevel() >= level || logger.hookLevel() >= level
}

// SetFormatter sets the logger formatter.
func (logger *Logger) SetFormatter(formatter Formatter) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Formatter = formatter
}

// SetOutput sets the logger output.
func (logger *Logger) SetOutput(output io.Writer) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Out = output
}

func (logger *Logger) SetReportCaller(reportCaller bool) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.ReportCaller = reportCaller
}

// ReplaceHooks replaces the logger hooks and returns the old ones
func (logger *Logger) ReplaceHooks(hooks LevelHooks) LevelHooks {
	logger.mu.Lock()
	oldHooks := logger.Hooks
	logger.Hooks = hooks
	logger.mu.Unlock()
	return oldHooks
}

// SetBufferPool sets the logger buffer pool.
func (logger *Logger) SetBufferPool(pool BufferPool) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.BufferPool = pool
}

func (logger *Logger) SetMaxAge(duration time.Duration) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.MaxAge = duration
}
