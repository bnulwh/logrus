package logrus

import (
	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"io"
	"path"
	"time"
)

func createFileLogger(level, logPath string) (*rotatelogs.RotateLogs, error) {
	prefix := ""
	if len(level) > 0 {
		prefix = "." + level
	}
	return rotatelogs.New(
		logPath+prefix+".%Y%m%d%H%M.log",
		rotatelogs.WithLinkName(logPath+prefix+".log"),
		rotatelogs.WithMaxAge(GetMaxAge()),
		rotatelogs.WithRotationTime(time.Hour),
	)
}

func ConfigLocalFileSystemLogger(logPath, logFileName string) {
	baseLogPath := path.Join(logPath, logFileName)
	debugWriter, err := createFileLogger("debug", baseLogPath)
	infoWriter, err := createFileLogger("info", baseLogPath)
	warnWriter, err := createFileLogger("warn", baseLogPath)
	errorWriter, err := createFileLogger("error", baseLogPath)
	commonWriter, err := createFileLogger("", baseLogPath)
	multiErrorWriter := io.MultiWriter(errorWriter, commonWriter)
	if err != nil {
		Errorf("config local file system logger error: %+v", errors.WithStack(err))
	}
	lfHook := newLocalFileSystemHook(WriterMap{
		DebugLevel: io.MultiWriter(debugWriter, commonWriter),
		InfoLevel:  io.MultiWriter(infoWriter, commonWriter),
		WarnLevel:  io.MultiWriter(warnWriter, commonWriter),
		ErrorLevel: multiErrorWriter,
		FatalLevel: multiErrorWriter,
		PanicLevel: multiErrorWriter,
	}, &SimpleFormatter{})

	AddHook(lfHook)
}
