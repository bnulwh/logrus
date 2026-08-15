package logrus

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type SimpleFormatter struct {
	Colored bool
}

// simpleTimeFormat is the timestamp layout used for every formatted line.
const simpleTimeFormat = "2006-01-02 15:04:05.000"

// simplePaddedLevels pre-formats each level to a fixed width of 7 characters
// (mirrors fmt.Sprintf("%7s", level)), so the hot path avoids Sprintf and a
// level->string lookup. Indexed by Level value (PanicLevel = 0 .. TraceLevel = 6).
var simplePaddedLevels = [...]string{
	"  panic", // PanicLevel
	"  fatal", // FatalLevel
	"  error", // ErrorLevel
	"warning", // WarnLevel
	"   info", // InfoLevel
	"  debug", // DebugLevel
	"  trace", // TraceLevel
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
	b.WriteByte('[')
	b.Write(entry.Time.AppendFormat(nil, simpleTimeFormat))
	b.WriteString("] [")
	levelText := " unknown"
	if int(entry.Level) < len(simplePaddedLevels) {
		levelText = simplePaddedLevels[entry.Level]
	}
	b.WriteString(levelText)
	b.WriteString("] ")
	// Caller is only populated by entry.log when ReportCaller was true at log
	// time (read under the logger mutex), so checking it here avoids an unsynchronized
	// read of Logger.ReportCaller from the hot path (see TestEntryReportCallerRace).
	if entry.Caller != nil {
		b.WriteString("[ ")
		b.WriteString(filepath.Base(entry.Caller.File))
		b.WriteString(" : ")
		b.WriteString(strconv.Itoa(entry.Caller.Line))
		b.WriteString(" : ")
		b.WriteString(getFuncName(entry.Caller.Func))
		b.WriteString("() ] : ")
	}
	b.WriteString(
		entry.Message,
	)
	if f.Colored {
		b.WriteString("\x1b[0m")
	}
	b.WriteByte('\n')
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
