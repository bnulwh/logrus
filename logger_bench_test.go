package logrus

import (
	"io/ioutil"
	"os"
	"testing"
)

func BenchmarkDummyLogger(b *testing.B) {
	nullf, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err != nil {
		b.Fatalf("%v", err)
	}
	defer nullf.Close()
	doLoggerBenchmark(b, nullf, &TextFormatter{DisableColors: true}, smallFields)
}

func BenchmarkDummyLoggerNoLock(b *testing.B) {
	nullf, err := os.OpenFile(os.DevNull, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		b.Fatalf("%v", err)
	}
	defer nullf.Close()
	doLoggerBenchmarkNoLock(b, nullf, &TextFormatter{DisableColors: true}, smallFields)
}

func doLoggerBenchmark(b *testing.B, out *os.File, formatter Formatter, fields Fields) {
	logger := Logger{
		Out:          out,
		ConsoleLevel: InfoLevel,
		HookLevel:    InfoLevel,
		Formatter:    formatter,
	}
	entry := logger.WithFields(fields)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			entry.Info("aaa")
		}
	})
}

func doLoggerBenchmarkNoLock(b *testing.B, out *os.File, formatter Formatter, fields Fields) {
	logger := Logger{
		Out:          out,
		ConsoleLevel: InfoLevel,
		HookLevel:    InfoLevel,
		Formatter:    formatter,
	}
	logger.SetNoLock()
	entry := logger.WithFields(fields)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			entry.Info("aaa")
		}
	})
}

func BenchmarkLoggerJSONFormatter(b *testing.B) {
	doLoggerBenchmarkWithFormatter(b, &JSONFormatter{})
}

func BenchmarkLoggerTextFormatter(b *testing.B) {
	doLoggerBenchmarkWithFormatter(b, &TextFormatter{})
}

func doLoggerBenchmarkWithFormatter(b *testing.B, f Formatter) {
	b.SetParallelism(100)
	log := New()
	log.Formatter = f
	log.Out = ioutil.Discard
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			log.
				WithField("foo1", "bar1").
				WithField("foo2", "bar2").
				Info("this is a dummy log")
		}
	})
}

// discardWriter is an allocation-free io.Writer for the direct-logger
// benchmarks below.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// BenchmarkDirectInfo benchmarks the most common path — calling a logging
// method straight on the Logger (no WithField chain) — which exercises the
// pooled-entry reuse path in Entry.log.
func BenchmarkDirectInfo(b *testing.B) {
	logger := &Logger{
		Out:          discardWriter{},
		ConsoleLevel: InfoLevel,
		HookLevel:    InfoLevel,
		Formatter:    &SimpleFormatter{},
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("hello")
		}
	})
}

// BenchmarkDirectInfoCallerTracing benchmarks the same path with caller
// tracing enabled (the fork's default in New()).
func BenchmarkDirectInfoCallerTracing(b *testing.B) {
	logger := &Logger{
		Out:          discardWriter{},
		ConsoleLevel: InfoLevel,
		HookLevel:    InfoLevel,
		Formatter:    &SimpleFormatter{},
		ReportCaller: true,
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("hello")
		}
	})
}
