package events

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// LogFile buffers text log lines for background file I/O. Writes block when
// the 256-entry queue is full. The events.jsonl filename does not imply JSON.
// The queue is drained before Close returns; the current file and one 5 MiB
// rotated file are retained.
type LogFile struct {
	stateMu sync.Mutex
	queueMu sync.Mutex
	fileMu  sync.Mutex
	file    *os.File
	path    string
	size    int64
	queue   chan []byte
	wg      sync.WaitGroup
	closed  bool
	err     error
}

func Open(dir string) (*LogFile, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "events.jsonl")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	w := &LogFile{file: f, path: p, size: info.Size(), queue: make(chan []byte, 256)}
	w.wg.Add(1)
	go w.consume()
	return w, nil
}

func (w *LogFile) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	b := append([]byte(nil), p...)
	// Holding queueMu prevents Close from closing the channel underneath this
	// send. The consumer never takes queueMu, so a full queue still drains.
	w.queueMu.Lock()
	defer w.queueMu.Unlock()
	if w.closed {
		return 0, fmt.Errorf("log file is closed")
	}
	w.queue <- b
	return len(p), nil
}

func (w *LogFile) consume() {
	defer w.wg.Done()
	for p := range w.queue {
		w.writeChunk(p)
	}
	w.fileMu.Lock()
	err := w.file.Close()
	w.fileMu.Unlock()
	if err != nil {
		w.setError(err)
	}
}

func (w *LogFile) writeChunk(p []byte) {
	w.fileMu.Lock()
	defer w.fileMu.Unlock()
	if w.size+int64(len(p)) > 5*1024*1024 {
		if err := w.file.Close(); err != nil {
			w.setError(err)
			return
		}
		if err := os.Rename(w.path, w.path+".1"); err != nil {
			w.setError(err)
			return
		}
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			w.setError(err)
			return
		}
		w.file, w.size = f, 0
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	if err != nil {
		w.setError(err)
	}
}

func (w *LogFile) setError(err error) {
	w.stateMu.Lock()
	if w.err == nil {
		w.err = err
	}
	w.stateMu.Unlock()
}

func (w *LogFile) Close() error {
	w.queueMu.Lock()
	if w.closed {
		w.queueMu.Unlock()
		w.stateMu.Lock()
		err := w.err
		w.stateMu.Unlock()
		return err
	}
	w.closed = true
	close(w.queue)
	w.queueMu.Unlock()
	w.wg.Wait()
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	return w.err
}
