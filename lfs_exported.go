package logrus

import (
	"fmt"
	"io"
	"path/filepath"
	"time"
)

func createRotatingWriter(level, logPath string) (*RotatingFileWriter, error) {
	prefix := ""
	if len(level) > 0 {
		prefix = "." + level
	}
	dir := filepath.Dir(logPath)
	baseName := filepath.Base(logPath) + prefix
	linkName := filepath.Base(logPath) + prefix + ".log"
	return NewRotatingFileWriter(RotatingFileConfig{
		Dir:      dir,
		BaseName: baseName,
		Ext:      ".log",
		Rotation: time.Hour,
		MaxAge:   GetMaxAge(),
		LinkName: linkName,
	})
}

func ConfigLocalFileSystemLogger(logPath, logFileName string) {
	baseLogPath := filepath.Join(logPath, logFileName)
	debugWriter, err := createRotatingWriter("debug", baseLogPath)
	if err != nil {
		Errorf("config local file system logger error: %v", fmt.Errorf("create debug writer: %w", err))
		return
	}
	infoWriter, err := createRotatingWriter("info", baseLogPath)
	if err != nil {
		Errorf("config local file system logger error: %v", fmt.Errorf("create info writer: %w", err))
		return
	}
	warnWriter, err := createRotatingWriter("warn", baseLogPath)
	if err != nil {
		Errorf("config local file system logger error: %v", fmt.Errorf("create warn writer: %w", err))
		return
	}
	errorWriter, err := createRotatingWriter("error", baseLogPath)
	if err != nil {
		Errorf("config local file system logger error: %v", fmt.Errorf("create error writer: %w", err))
		return
	}
	commonWriter, err := createRotatingWriter("", baseLogPath)
	if err != nil {
		Errorf("config local file system logger error: %v", fmt.Errorf("create common writer: %w", err))
		return
	}
	multiErrorWriter := io.MultiWriter(errorWriter, commonWriter)
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
