package logrus

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type SimpleFormatter struct {
	Colored bool
}

func (f *SimpleFormatter) Format(entry *Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	if f.Colored {
		switch entry.Level {
		case TraceLevel, DebugLevel:
			b.WriteString("\x1b[34;1m")
		case InfoLevel:
			b.WriteString("\x1b[32;1m")
		case WarnLevel:
			b.WriteString("\x1b[35;1m")
		case ErrorLevel, FatalLevel, PanicLevel:
			b.WriteString("\x1b[31;1m")
		}
	}
	b.WriteString(fmt.Sprintf("[%s] [%8s] ",
		entry.Time.Format("2006-01-02 15:04:05.000"),
		entry.Level.String()))
	if entry.Logger.ReportCaller && entry.Caller != nil {
		b.WriteString(fmt.Sprintf("[ %s : %d %s ]",
			filepath.Base(entry.Caller.File),
			entry.Caller.Line,
			getFuncName(entry.Caller.Func),
		))
	}
	b.WriteString(
		entry.Message,
	)
	if f.Colored {
		b.WriteString("\x1b[0m")
	}
	b.WriteString("\n")
	return b.Bytes(), nil
}

func getFuncName(f *runtime.Func) string {
	if f != nil {
		fullFnName := f.Name()
		pos := strings.LastIndex(fullFnName, ".")
		return fullFnName[pos+1:]
	}
	return ""
}
