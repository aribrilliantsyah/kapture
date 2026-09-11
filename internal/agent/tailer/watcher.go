package tailer

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a directory for new, modified, and deleted container log files.
type Watcher struct {
	dir      string
	w        *fsnotify.Watcher
	onCreate func(path string)
	onModify func(path string)
	onRemove func(path string)
	done     chan struct{}
}

// WatcherOpts configures the watcher.
type WatcherOpts struct {
	Dir      string
	OnCreate func(path string)
	OnModify func(path string)
	OnRemove func(path string)
}

// NewWatcher creates a directory watcher.
func NewWatcher(opts WatcherOpts) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		dir:      opts.Dir,
		w:        w,
		onCreate: opts.OnCreate,
		onModify: opts.OnModify,
		onRemove: opts.OnRemove,
		done:     make(chan struct{}),
	}, nil
}

// Start begins watching the directory.
func (w *Watcher) Start() error {
	// Add directory to watcher
	if err := w.w.Add(w.dir); err != nil {
		return err
	}

	// Process existing files
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			path := filepath.Join(w.dir, e.Name())
			if w.onCreate != nil {
				w.onCreate(path)
			}
		}
	}

	go w.loop()
	return nil
}

// Stop stops watching.
func (w *Watcher) Stop() {
	close(w.done)
	w.w.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.w.Events:
			if !ok {
				return
			}
			if !isLogFile(filepath.Base(event.Name)) {
				continue
			}

			switch {
			case event.Op&fsnotify.Create != 0:
				slog.Debug("new log file", "path", event.Name)
				if w.onCreate != nil {
					w.onCreate(event.Name)
				}
			case event.Op&fsnotify.Write != 0:
				if w.onModify != nil {
					w.onModify(event.Name)
				}
			case event.Op&fsnotify.Remove != 0, event.Op&fsnotify.Rename != 0:
				slog.Debug("log file removed", "path", event.Name)
				if w.onRemove != nil {
					w.onRemove(event.Name)
				}
			}

		case err, ok := <-w.w.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)
		}
	}
}

func isLogFile(name string) bool {
	return strings.HasSuffix(name, ".log")
}
