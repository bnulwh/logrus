package logrus

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// qualified package name, cached at first use
	logrusPackage string

	// Positions in the call stack when tracing to report the calling method
	minimumCallerDepth int

	// Used for caller information initialisation
	callerInitOnce sync.Once
)

// callerPcsPool recycles the PC scratch buffer used by getCaller. The buffer
// escapes to the heap because runtime.CallersFrames retains it, so pooling
// avoids a ~200B allocation on every caller-tracing log (ReportCaller is on
// by default in this fork).
var callerPcsPool = sync.Pool{
	New: func() interface{} {
		s := make([]uintptr, maximumCallerDepth)
		return &s
	},
}

const (
	maximumCallerDepth int = 25
	knownLogrusFrames  int = 4
)

func init() {
	// start at the bottom of the stack before the package-name cache is primed
	minimumCallerDepth = 1
}

// Defines the key when adding errors using WithError.
var ErrorKey = "error"

// An entry is the final or intermediate Logrus logging entry. It contains all
// the fields passed with WithField{,s}. It's finally logged when Trace, Debug,
// Info, Warn, Error, Fatal or Panic is called on it. These objects can be
// reused and passed around as much as you wish to avoid field duplication.
type Entry struct {
	Logger *Logger

	// Contains all the fields set by the user.
	Data Fields

	// Time at which the log entry was created
	Time time.Time

	// Level the log entry was logged at: Trace, Debug, Info, Warn, Error, Fatal or Panic
	// This field will be set on entry firing and the value will be equal to the one in Logger struct field.
	Level        Level
	ConsoleLevel Level
	HookLevel    Level

	// Calling method, with package name
	Caller *runtime.Frame

	// Message passed to Trace, Debug, Info, Warn, Error, Fatal or Panic
	Message string

	// When formatter is called in entry.log(), a Buffer may be set to entry
	Buffer *bytes.Buffer

	// Contains the context set by the user. Useful for hook processing etc.
	Context context.Context

	// err may contain a field formatting error
	err string

	// pooled marks entries handed out by Logger.newEntry. They are single-use
	// scratch objects that get cleared and returned to the logger's entry pool
	// after logging, so log() may reuse them in place instead of Dup'ing.
	pooled bool
}

func NewEntry(logger *Logger) *Entry {
	return &Entry{
		Logger: logger,
		// Default is three fields, plus one optional.  Give a little extra room.
		Data:         make(Fields, 6),
		ConsoleLevel: logger.ConsoleLevel,
		HookLevel:    logger.HookLevel,
	}
}

func (entry *Entry) Dup() *Entry {
	data := make(Fields, len(entry.Data))
	for k, v := range entry.Data {
		data[k] = v
	}
	return &Entry{Logger: entry.Logger,
		Data:         data,
		Time:         entry.Time,
		Context:      entry.Context,
		err:          entry.err,
		ConsoleLevel: entry.Logger.ConsoleLevel,
		HookLevel:    entry.Logger.HookLevel,
	}
}

// Returns the bytes representation of this entry from the formatter.
func (entry *Entry) Bytes() ([]byte, error) {
	return entry.Logger.Formatter.Format(entry)
}

// Returns the string representation from the reader and ultimately the
// formatter.
func (entry *Entry) String() (string, error) {
	serialized, err := entry.Bytes()
	if err != nil {
		return "", err
	}
	str := string(serialized)
	return str, nil
}

// Add an error as single field (using the key defined in ErrorKey) to the Entry.
func (entry *Entry) WithError(err error) *Entry {
	return entry.WithField(ErrorKey, err)
}

// Add a context to the Entry.
func (entry *Entry) WithContext(ctx context.Context) *Entry {
	dataCopy := make(Fields, len(entry.Data))
	for k, v := range entry.Data {
		dataCopy[k] = v
	}
	return &Entry{Logger: entry.Logger, Data: dataCopy, Time: entry.Time, err: entry.err, Context: ctx}
}

// Add a single field to the Entry.
func (entry *Entry) WithField(key string, value interface{}) *Entry {
	return entry.WithFields(Fields{key: value})
}

// Add a map of fields to the Entry.
func (entry *Entry) WithFields(fields Fields) *Entry {
	data := make(Fields, len(entry.Data)+len(fields))
	for k, v := range entry.Data {
		data[k] = v
	}
	fieldErr := entry.err
	for k, v := range fields {
		isErrField := false
		if t := reflect.TypeOf(v); t != nil {
			switch {
			case t.Kind() == reflect.Func, t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Func:
				isErrField = true
			}
		}
		if isErrField {
			tmp := fmt.Sprintf("can not add field %q", k)
			if fieldErr != "" {
				fieldErr = entry.err + ", " + tmp
			} else {
				fieldErr = tmp
			}
		} else {
			data[k] = v
		}
	}
	return &Entry{Logger: entry.Logger, Data: data, Time: entry.Time, err: fieldErr, Context: entry.Context}
}

// Overrides the time of the Entry.
func (entry *Entry) WithTime(t time.Time) *Entry {
	dataCopy := make(Fields, len(entry.Data))
	for k, v := range entry.Data {
		dataCopy[k] = v
	}
	return &Entry{Logger: entry.Logger, Data: dataCopy, Time: t, err: entry.err, Context: entry.Context}
}

// getPackageName reduces a fully qualified function name to the package name
// There really ought to be to be a better way...
func getPackageName(f string) string {
	for {
		lastPeriod := strings.LastIndex(f, ".")
		lastSlash := strings.LastIndex(f, "/")
		if lastPeriod > lastSlash {
			f = f[:lastPeriod]
		} else {
			break
		}
	}

	return f
}

// getCaller retrieves the name of the first non-logrus calling function
func getCaller() *runtime.Frame {
	// cache this package's fully-qualified name
	callerInitOnce.Do(func() {
		pcs := make([]uintptr, maximumCallerDepth)
		_ = runtime.Callers(0, pcs)

		// dynamic get the package name and the minimum caller depth
		for i := 0; i < maximumCallerDepth; i++ {
			funcName := runtime.FuncForPC(pcs[i]).Name()
			if strings.Contains(funcName, "getCaller") {
				logrusPackage = getPackageName(funcName)
				break
			}
		}

		minimumCallerDepth = knownLogrusFrames
	})

	// Restrict the lookback frames to avoid runaway lookups
	pcsPtr := callerPcsPool.Get().(*[]uintptr)
	pcs := *pcsPtr
	defer callerPcsPool.Put(pcsPtr)
	depth := runtime.Callers(minimumCallerDepth, pcs)
	frames := runtime.CallersFrames(pcs[:depth])

	for f, again := frames.Next(); again; f, again = frames.Next() {
		pkg := getPackageName(f.Function)

		// If the caller isn't part of this package, we're done
		if pkg != logrusPackage {
			return &f //nolint:scopelint
		}
	}

	// if we got here, we failed to find the caller's context
	return nil
}

func (entry Entry) HasCaller() (has bool) {
	return entry.Logger != nil &&
		entry.Logger.ReportCaller &&
		entry.Caller != nil
}

func (entry *Entry) log(level Level, msg string) {
	var buffer *bytes.Buffer

	// Snapshot the mutable logger config under a single lock acquisition:
	// caller reporting flag, buffer pool, and (only when hooks can fire at this
	// level) a shallow copy of the hooks map. Hooks are fired after the lock is
	// released, so hook execution never blocks concurrent loggers.
	entry.Logger.mu.Lock()
	reportCaller := entry.Logger.ReportCaller
	bufPool := entry.getBufferPool()
	// Note: read the logger's HookLevel, not entry.HookLevel — Entry.WithFields
	// does not propagate the level onto the entry it returns, and the original
	// code (via Dup) also read the logger field directly.
	hooksFire := entry.Logger.HookLevel >= level && len(entry.Logger.Hooks) > 0
	var tmpHooks LevelHooks
	if hooksFire {
		tmpHooks = make(LevelHooks, len(entry.Logger.Hooks))
		for k, v := range entry.Logger.Hooks {
			tmpHooks[k] = v
		}
	}
	entry.Logger.mu.Unlock()

	// Pooled entries from Logger.newEntry are single-use scratch objects that
	// get cleared and returned to the pool after logging, so when no hook can
	// observe them, log() reuses the entry in place and skips the Dup (one
	// Entry and one Fields map allocation per log call). User-facing entries
	// (WithField chains, reused entries), panic logs (the entry becomes the
	// panic value) and hook-enabled logs keep the original snapshot semantics.
	newEntry := entry
	if !entry.pooled || level <= PanicLevel || tmpHooks != nil {
		newEntry = entry.Dup()
	}

	if newEntry.Time.IsZero() {
		newEntry.Time = time.Now()
	}

	newEntry.Level = level
	newEntry.Message = msg

	if reportCaller {
		newEntry.Caller = getCaller()
	}
	if tmpHooks != nil {
		if err := tmpHooks.Fire(level, newEntry); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fire hook: %v\n", err)
		}
	}
	buffer = bufPool.Get()
	defer func() {
		newEntry.Buffer = nil
		buffer.Reset()
		bufPool.Put(buffer)
	}()
	buffer.Reset()
	newEntry.Buffer = buffer
	if newEntry.ConsoleLevel >= level {
		newEntry.write()
	}

	newEntry.Buffer = nil

	// To avoid Entry#log() returning a value that only would make sense for
	// panic() to use in Entry#Panic(), we avoid the allocation by checking
	// directly here.
	if level <= PanicLevel {
		panic(newEntry)
	}
}

func (entry *Entry) getBufferPool() (pool BufferPool) {
	if entry.Logger.BufferPool != nil {
		return entry.Logger.BufferPool
	}
	return bufferPool
}

func (entry *Entry) write() {
	serialized, err := entry.Logger.Formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to obtain reader, %v\n", err)
		return
	}
	entry.Logger.mu.Lock()
	defer entry.Logger.mu.Unlock()
	if _, err := entry.Logger.Out.Write(serialized); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
	}
}

func (entry *Entry) Log(level Level, args ...interface{}) {
	if entry.Logger.IsLevelEnabled(level) {
		entry.log(level, sprintMsg(args...))
	}
}

// Entry Printf family functions

func (entry *Entry) Logf(level Level, format string, args ...interface{}) {
	if entry.Logger.IsLevelEnabled(level) {
		entry.log(level, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Trace(args ...interface{}) {
	//entry.Log(TraceLevel, args...)
	if entry.Logger.IsLevelEnabled(TraceLevel) {
		entry.log(TraceLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Debug(args ...interface{}) {
	//entry.Log(DebugLevel, args...)
	if entry.Logger.IsLevelEnabled(DebugLevel) {
		entry.log(DebugLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Print(args ...interface{}) {
	//entry.Info(args...)
	if entry.Logger.IsLevelEnabled(InfoLevel) {
		entry.log(InfoLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Info(args ...interface{}) {
	//entry.Log(InfoLevel, args...)
	if entry.Logger.IsLevelEnabled(InfoLevel) {
		entry.log(InfoLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Warn(args ...interface{}) {
	//entry.Log(WarnLevel, args...)
	if entry.Logger.IsLevelEnabled(WarnLevel) {
		entry.log(WarnLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Warning(args ...interface{}) {
	//entry.Warn(args...)
	if entry.Logger.IsLevelEnabled(WarnLevel) {
		entry.log(WarnLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Error(args ...interface{}) {
	//entry.Log(ErrorLevel, args...)
	if entry.Logger.IsLevelEnabled(ErrorLevel) {
		entry.log(ErrorLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Fatal(args ...interface{}) {
	//entry.Log(FatalLevel, args...)
	if entry.Logger.IsLevelEnabled(FatalLevel) {
		entry.log(FatalLevel, sprintMsg(args...))
	}
	entry.Logger.Exit(1)
}

func (entry *Entry) Panic(args ...interface{}) {
	//entry.Log(PanicLevel, args...)
	if entry.Logger.IsLevelEnabled(PanicLevel) {
		entry.log(PanicLevel, sprintMsg(args...))
	}
}

func (entry *Entry) Tracef(format string, args ...interface{}) {
	//entry.Logf(TraceLevel, format, args...)
	if entry.Logger.IsLevelEnabled(TraceLevel) {
		entry.log(TraceLevel, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Debugf(format string, args ...interface{}) {
	//entry.Logf(DebugLevel, format, args...)
	if entry.Logger.IsLevelEnabled(DebugLevel) {
		entry.log(DebugLevel, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Infof(format string, args ...interface{}) {
	//entry.Logf(InfoLevel, format, args...)
	if entry.Logger.IsLevelEnabled(InfoLevel) {
		entry.log(InfoLevel, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Printf(format string, args ...interface{}) {
	//entry.Infof(format, args...)
	if entry.Logger.IsLevelEnabled(InfoLevel) {
		entry.log(InfoLevel, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Warnf(format string, args ...interface{}) {
	//entry.Logf(WarnLevel, format, args...)
	if entry.Logger.IsLevelEnabled(WarnLevel) {
		entry.log(WarnLevel, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Warningf(format string, args ...interface{}) {
	//entry.Warnf(format, args...)
	if entry.Logger.IsLevelEnabled(WarnLevel) {
		entry.log(WarnLevel, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Errorf(format string, args ...interface{}) {
	//entry.Logf(ErrorLevel, format, args...)
	if entry.Logger.IsLevelEnabled(ErrorLevel) {
		entry.log(ErrorLevel, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Fatalf(format string, args ...interface{}) {
	//entry.Logf(FatalLevel, format, args...)
	if entry.Logger.IsLevelEnabled(FatalLevel) {
		entry.log(FatalLevel, fmt.Sprintf(format, args...))
	}
	entry.Logger.Exit(1)
}

func (entry *Entry) Panicf(format string, args ...interface{}) {
	//entry.Logf(PanicLevel, format, args...)
	if entry.Logger.IsLevelEnabled(PanicLevel) {
		entry.log(PanicLevel, fmt.Sprintf(format, args...))
	}
}

// Entry Println family functions

func (entry *Entry) Logln(level Level, args ...interface{}) {
	if entry.Logger.IsLevelEnabled(level) {
		entry.log(level, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Traceln(args ...interface{}) {
	//entry.Logln(TraceLevel, args...)
	if entry.Logger.IsLevelEnabled(TraceLevel) {
		entry.log(TraceLevel, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Debugln(args ...interface{}) {
	//entry.Logln(DebugLevel, args...)
	if entry.Logger.IsLevelEnabled(DebugLevel) {
		entry.log(DebugLevel, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Infoln(args ...interface{}) {
	entry.Logln(InfoLevel, args...)
}

func (entry *Entry) Println(args ...interface{}) {
	//entry.Infoln(args...)
	if entry.Logger.IsLevelEnabled(InfoLevel) {
		entry.log(InfoLevel, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Warnln(args ...interface{}) {
	//entry.Logln(WarnLevel, args...)
	if entry.Logger.IsLevelEnabled(WarnLevel) {
		entry.log(WarnLevel, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Warningln(args ...interface{}) {
	//entry.Warnln(args...)
	if entry.Logger.IsLevelEnabled(WarnLevel) {
		entry.log(WarnLevel, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Errorln(args ...interface{}) {
	//entry.Logln(ErrorLevel, args...)
	if entry.Logger.IsLevelEnabled(ErrorLevel) {
		entry.log(ErrorLevel, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Fatalln(args ...interface{}) {
	//entry.Logln(FatalLevel, args...)
	if entry.Logger.IsLevelEnabled(FatalLevel) {
		entry.log(FatalLevel, entry.sprintlnn(args...))
	}
	entry.Logger.Exit(1)
}

func (entry *Entry) Panicln(args ...interface{}) {
	//entry.Logln(PanicLevel, args...)
	if entry.Logger.IsLevelEnabled(PanicLevel) {
		entry.log(PanicLevel, entry.sprintlnn(args...))
	}
}

// sprintMsg renders args exactly like fmt.Sprint, but with a fast path for the
// extremely common single-string case (log.Info("plain message")) that avoids
// fmt's reflection machinery and a string allocation.
func sprintMsg(args ...interface{}) string {
	if len(args) == 1 {
		if s, ok := args[0].(string); ok {
			return s
		}
	}
	return fmt.Sprint(args...)
}

// Sprintlnn => Sprint no newline. This is to get the behavior of how
// fmt.Sprintln where spaces are always added between operands, regardless of
// their type. Instead of vendoring the Sprintln implementation to spare a
// string allocation, we do the simplest thing.
func (entry *Entry) sprintlnn(args ...interface{}) string {
	if len(args) == 1 {
		if s, ok := args[0].(string); ok {
			return s
		}
	}
	msg := fmt.Sprintln(args...)
	return msg[:len(msg)-1]
}
