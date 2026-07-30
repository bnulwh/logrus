package logrus

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var defaultFormatter = &TextFormatter{DisableColors: true}

type LfsHook struct {
	writers   WriterMap
	levels    []Level
	lock      *sync.Mutex
	formatter Formatter

	defaultWriter    io.Writer
	hasDefaultWriter bool
}

func newLocalFileSystemHook(output WriterMap, formatter Formatter) *LfsHook {
	hook := &LfsHook{
		lock:    new(sync.Mutex),
		writers: output,
	}
	hook.SetFormatter(formatter)
	for level := range output {
		hook.levels = append(hook.levels, level)
	}
	return hook
}

func (hook *LfsHook) SetFormatter(formatter Formatter) {
	hook.lock.Lock()
	defer hook.lock.Unlock()
	if formatter == nil {
		formatter = defaultFormatter
	} else {
		if tf, ok := formatter.(*TextFormatter); ok {
			tf.DisableColors = true
		}
	}
	hook.formatter = formatter
}

func (hook *LfsHook) SetDefaultWriter(defaultWriter io.Writer) {
	hook.lock.Lock()
	defer hook.lock.Unlock()
	hook.defaultWriter = defaultWriter
	hook.hasDefaultWriter = true
}

func (hook *LfsHook) Fire(entry *Entry) error {
	hook.lock.Lock()
	defer hook.lock.Unlock()

	var writer io.Writer
	var ok bool
	if writer, ok = hook.writers[entry.Level]; !ok {
		if hook.hasDefaultWriter {
			writer = hook.defaultWriter
		} else {
			return nil
		}
	}

	msg, err := hook.formatter.Format(entry)
	if err != nil {
		log.Println("failed to generate string for entry:", err)
		return err
	}
	_, err = writer.Write(msg)
	return err
}

func (hook *LfsHook) Levels() []Level {
	return AllLevels
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		return os.MkdirAll(dir, os.ModePerm)
	}
	return nil
}
