package events

import (
	"os"
	"path/filepath"
	"sync"
)

// LogFile keeps the current log and one previous file, at most roughly 10 MiB.
type LogFile struct {
	mu   sync.Mutex
	file *os.File
	path string
	size int64
}

func Open(dir string) (*LogFile, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "events.log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &LogFile{file: f, path: p, size: info.Size()}, nil
}
func (w *LogFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > 5*1024*1024 {
		if err := w.file.Close(); err != nil {
			return 0, err
		}
		if err := os.Rename(w.path, w.path+".1"); err != nil {
			return 0, err
		}
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return 0, err
		}
		w.file = f
		w.size = 0
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}
func (w *LogFile) Close() error { w.mu.Lock(); defer w.mu.Unlock(); return w.file.Close() }
