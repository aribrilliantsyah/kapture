package tailer

import (
	"log/slog"
	"sync"

	"github.com/ordinary/k8s-log-catcher/internal/agent/enricher"
	"github.com/ordinary/k8s-log-catcher/internal/config"
	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/storage"
)

// Tailer manages all file readers and the directory watcher.
type Tailer struct {
	cfg     config.AgentConfig
	node    string
	store   storage.Store
	watcher *Watcher
	readers map[string]*Reader
	out     chan model.LogEntry
	exclude map[string]bool
	mu      sync.Mutex
	sem     chan struct{}
	done    chan struct{}
}

// New creates a new Tailer.
func New(cfg config.AgentConfig, node string, store storage.Store) *Tailer {
	excl := make(map[string]bool)
	for _, ns := range cfg.Exclude.Namespaces {
		excl[ns] = true
	}

	return &Tailer{
		cfg:     cfg,
		node:    node,
		store:   store,
		readers: make(map[string]*Reader),
		out:     make(chan model.LogEntry, cfg.Collector.BatchSize*2),
		exclude: excl,
		sem:     make(chan struct{}, cfg.Collector.MaxGoroutines),
		done:    make(chan struct{}),
	}
}

// Out returns the channel where parsed log entries are emitted.
func (t *Tailer) Out() <-chan model.LogEntry {
	return t.out
}

// Start begins tailing log files and watching for changes.
func (t *Tailer) Start() error {
	w, err := NewWatcher(WatcherOpts{
		Dir:      t.cfg.LogPath,
		OnCreate: t.onFileCreate,
		OnModify: t.onFileModify,
		OnRemove: t.onFileRemove,
	})
	if err != nil {
		return err
	}
	t.watcher = w
	return w.Start()
}

// Stop gracefully shuts down all readers and the watcher.
func (t *Tailer) Stop() {
	close(t.done)
	if t.watcher != nil {
		t.watcher.Stop()
	}
	t.mu.Lock()
	for _, r := range t.readers {
		r.Stop()
	}
	t.mu.Unlock()
}

func (t *Tailer) onFileCreate(path string) {
	meta, ok := enricher.ParseFilename(path)
	if !ok {
		slog.Warn("cannot parse log filename", "path", path)
		return
	}

	// Check exclusion
	if t.exclude[meta.Namespace] {
		return
	}

	t.mu.Lock()
	if _, exists := t.readers[path]; exists {
		t.mu.Unlock()
		return
	}

	// Respect goroutine limit
	select {
	case t.sem <- struct{}{}:
	default:
		slog.Warn("max goroutines reached, skipping file", "path", path)
		t.mu.Unlock()
		return
	}

	reader := NewReader(ReaderOpts{
		Path:    path,
		Meta:    meta,
		Node:    t.node,
		Store:   t.store,
		Out:     t.out,
		BufSize: t.cfg.Collector.BufferSize,
	})
	t.readers[path] = reader
	t.mu.Unlock()

	reader.Start()
}

func (t *Tailer) onFileModify(path string) {
	t.mu.Lock()
	reader, ok := t.readers[path]
	t.mu.Unlock()
	if !ok {
		// File was created before watcher started, create reader now
		t.onFileCreate(path)
		t.mu.Lock()
		reader, ok = t.readers[path]
		t.mu.Unlock()
		if !ok {
			return
		}
	}
	go reader.ReadNew()
}

func (t *Tailer) onFileRemove(path string) {
	t.mu.Lock()
	reader, ok := t.readers[path]
	if ok {
		reader.Stop()
		delete(t.readers, path)
		<-t.sem // release goroutine slot
	}
	t.mu.Unlock()
}
