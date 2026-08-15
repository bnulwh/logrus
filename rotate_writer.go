package logrus

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RotatingFileWriter struct {
	mu sync.Mutex

	dir         string
	baseName    string
	ext         string
	rotation    time.Duration
	maxAge      time.Duration
	maxSize     int64
	linkName    string
	formatter   *time.Time

	file       *os.File
	fileSize   int64
	openedAt   time.Time
	cleanupCh  chan struct{}
	closed     bool
}

type RotatingFileConfig struct {
	Dir         string
	BaseName    string
	Ext         string
	Rotation    time.Duration
	MaxAge      time.Duration
	MaxSize     int64
	LinkName    string
}

func NewRotatingFileWriter(cfg RotatingFileConfig) (*RotatingFileWriter, error) {
	if cfg.Ext == "" {
		cfg.Ext = ".log"
	}
	if cfg.Rotation <= 0 {
		cfg.Rotation = time.Hour
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = time.Hour * 24 * 7
	}
	w := &RotatingFileWriter{
		dir:       cfg.Dir,
		baseName:  cfg.BaseName,
		ext:       cfg.Ext,
		rotation:  cfg.Rotation,
		maxAge:    cfg.MaxAge,
		maxSize:   cfg.MaxSize,
		linkName:  cfg.LinkName,
		cleanupCh: make(chan struct{}),
	}
	if err := os.MkdirAll(w.dir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory %s: %w", w.dir, err)
	}
	if err := w.openFile(time.Now()); err != nil {
		return nil, err
	}
	go w.cleanupLoop()
	return w, nil
}

func (w *RotatingFileWriter) rotatedFileName(t time.Time) string {
	ts := t.Format("200601021504")
	name := w.baseName + "." + ts + w.ext
	return filepath.Join(w.dir, name)
}

func (w *RotatingFileWriter) openFile(t time.Time) error {
	path := w.rotatedFileName(t)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat log file %s: %w", path, err)
	}
	w.file = f
	w.fileSize = stat.Size()
	w.openedAt = t.Truncate(w.rotation)

	if w.linkName != "" {
		linkPath := filepath.Join(w.dir, w.linkName)
		_ = os.Remove(linkPath)
		_ = os.Symlink(path, linkPath)
	}
	return nil
}

func (w *RotatingFileWriter) rotateIfNeeded(now time.Time) error {
	needRotate := false
	currentPeriod := now.Truncate(w.rotation)
	if !currentPeriod.Equal(w.openedAt) {
		needRotate = true
	}
	if w.maxSize > 0 && w.fileSize >= w.maxSize {
		needRotate = true
	}
	if !needRotate {
		return nil
	}
	if w.file != nil {
		_ = w.file.Close()
	}
	return w.openFile(now)
}

func (w *RotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}
	if err := w.rotateIfNeeded(time.Now()); err != nil {
		return 0, err
	}
	n, err := w.file.Write(p)
	w.fileSize += int64(n)
	return n, err
}

func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	close(w.cleanupCh)
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *RotatingFileWriter) cleanupLoop() {
	ticker := time.NewTicker(w.maxAge / 2)
	if ticker.C == nil {
		return
	}
	defer ticker.Stop()
	for {
		select {
		case <-w.cleanupCh:
			return
		case now := <-ticker.C:
			w.removeOldFiles(now.Add(-w.maxAge))
		}
	}
}

func (w *RotatingFileWriter) removeOldFiles(cutoff time.Time) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	prefix := w.baseName + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < len(prefix) || name[:len(prefix)] != prefix {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

var _ io.WriteCloser = (*RotatingFileWriter)(nil)
