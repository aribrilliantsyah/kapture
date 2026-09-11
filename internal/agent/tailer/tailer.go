package tailer

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/agent/enricher"
	"github.com/ordinary/k8s-log-catcher/internal/config"
	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/storage"
)

const (
	// /var/log/containers/*.log are symlinks into /var/log/pods, and inotify on
	// the directory never reports writes to symlink targets. Every reader is
	// therefore polled (one fstat + read per file); inotify only speeds up
	// file discovery and plain files.
	pollInterval   = time.Second
	rescanInterval = 30 * time.Second
)

// Tailer follows every container log file in the log directory.
type Tailer struct {
	cfg      config.AgentConfig
	node     string
	store    storage.Store
	out      chan model.LogEntry
	exclude  map[string]bool
	limiter  *rateLimiter
	resolver *enricher.Resolver

	mu      sync.Mutex
	readers map[string]*Reader

	watcher *Watcher
	wake    chan struct{}
	done    chan struct{}
	stopped chan struct{}
}

// New creates a new Tailer. resolver may be nil (workloads from pod names).
func New(cfg config.AgentConfig, node string, store storage.Store, resolver *enricher.Resolver) *Tailer {
	excl := make(map[string]bool)
	for _, ns := range cfg.Exclude.Namespaces {
		excl[ns] = true
	}
	return &Tailer{
		cfg:      cfg,
		node:     node,
		store:    store,
		out:      make(chan model.LogEntry, max(cfg.Collector.BatchSize*2, 256)),
		exclude:  excl,
		limiter:  newRateLimiter(cfg.Collector.RateLimit),
		resolver: resolver,
		readers:  make(map[string]*Reader),
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// Out returns the channel where parsed log entries are emitted.
func (t *Tailer) Out() <-chan model.LogEntry {
	return t.out
}

// Start registers existing files, then follows them in the background.
func (t *Tailer) Start() error {
	w, err := NewWatcher(WatcherOpts{
		Dir:      t.cfg.LogPath,
		OnCreate: t.add,
		OnModify: func(string) { t.poke() },
		OnRemove: t.remove,
	})
	if err != nil {
		return err
	}
	t.watcher = w
	if err := w.Start(); err != nil {
		return err
	}
	slog.Info("tailing container logs", "dir", t.cfg.LogPath, "files", t.count())
	go t.loop()
	return nil
}

// Stop halts polling and closes every file.
func (t *Tailer) Stop() {
	close(t.done)
	if t.watcher != nil {
		t.watcher.Stop()
	}
	<-t.stopped
	t.mu.Lock()
	for _, r := range t.readers {
		r.closeFile()
	}
	t.mu.Unlock()
}

func (t *Tailer) loop() {
	defer close(t.stopped)
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	rescan := time.NewTicker(rescanInterval)
	defer rescan.Stop()

	t.pollAll()
	for {
		select {
		case <-t.done:
			return
		case <-rescan.C:
			t.rescan()
		case <-poll.C:
		case <-t.wake:
		}
		t.pollAll()
	}
}

func (t *Tailer) pollAll() {
	t.mu.Lock()
	readers := make([]*Reader, 0, len(t.readers))
	for _, r := range t.readers {
		readers = append(readers, r)
	}
	t.mu.Unlock()

	for _, r := range readers {
		select {
		case <-t.done:
			return
		default:
			r.Poll()
		}
	}
}

// poke schedules an immediate poll.
func (t *Tailer) poke() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

func (t *Tailer) add(path string) {
	meta, ok := enricher.ParseFilename(path)
	if !ok {
		slog.Warn("cannot parse log filename", "path", path)
		return
	}
	if t.exclude[meta.Namespace] {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.readers[path]; exists {
		return
	}
	t.readers[path] = NewReader(ReaderOpts{
		Path:     path,
		Meta:     meta,
		Node:     t.node,
		Store:    t.store,
		Out:      t.out,
		Limiter:  t.limiter,
		Done:     t.done,
		BufSize:  t.cfg.Collector.BufferSize,
		Resolver: t.resolver,
	})
	slog.Debug("following log file", "path", path)
	t.poke()
}

// remove reads what is left of a deleted file (the open handle keeps it
// readable) and forgets it.
func (t *Tailer) remove(path string) {
	t.mu.Lock()
	r, ok := t.readers[path]
	delete(t.readers, path)
	t.mu.Unlock()
	if !ok {
		return
	}
	r.Close()
	if err := t.store.DeleteOffset(path); err != nil {
		slog.Warn("failed to delete offset", "path", path, "error", err)
	}
	slog.Debug("stopped following log file", "path", path)
}

// rescan catches files whose inotify events were missed.
func (t *Tailer) rescan() {
	entries, err := os.ReadDir(t.cfg.LogPath)
	if err != nil {
		slog.Warn("rescan failed", "dir", t.cfg.LogPath, "error", err)
		return
	}
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			path := filepath.Join(t.cfg.LogPath, e.Name())
			present[path] = true
			t.add(path)
		}
	}
	t.mu.Lock()
	var gone []string
	for path := range t.readers {
		if !present[path] {
			gone = append(gone, path)
		}
	}
	t.mu.Unlock()
	for _, path := range gone {
		t.remove(path)
	}
}

func (t *Tailer) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.readers)
}

// rateLimiter caps emitted lines per second. Lines over the limit wait (the
// file keeps them), they are never dropped. Only the poll loop uses it.
type rateLimiter struct {
	rate   float64
	tokens float64
	last   time.Time
}

func newRateLimiter(perSec int) *rateLimiter {
	if perSec <= 0 {
		return nil
	}
	return &rateLimiter{rate: float64(perSec), tokens: float64(perSec), last: time.Now()}
}

func (l *rateLimiter) take(done <-chan struct{}) {
	if l == nil {
		return
	}
	now := time.Now()
	l.tokens = min(l.rate, l.tokens+now.Sub(l.last).Seconds()*l.rate)
	l.last = now
	if l.tokens < 1 {
		wait := time.Duration((1 - l.tokens) / l.rate * float64(time.Second))
		select {
		case <-time.After(wait):
		case <-done:
		}
		l.last = time.Now()
		l.tokens = 1
	}
	l.tokens--
}
