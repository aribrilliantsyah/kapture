package tailer

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/ordinary/k8s-log-catcher/internal/agent/enricher"
	"github.com/ordinary/k8s-log-catcher/internal/agent/parser"
	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/storage"
)

// Reader tails a single log file and sends parsed entries to the output channel.
type Reader struct {
	path      string
	meta      enricher.FileMeta
	workload  enricher.Workload
	node      string
	store     storage.Store
	out       chan<- model.LogEntry
	bufSize   int
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
}

// ReaderOpts configures a file reader.
type ReaderOpts struct {
	Path    string
	Meta    enricher.FileMeta
	Node    string
	Store   storage.Store
	Out     chan<- model.LogEntry
	BufSize int
}

// NewReader creates a new file reader.
func NewReader(opts ReaderOpts) *Reader {
	wl := enricher.ExtractWorkload(opts.Meta.Pod)
	bufSize := opts.BufSize
	if bufSize <= 0 {
		bufSize = 4096
	}
	return &Reader{
		path:     opts.Path,
		meta:     opts.Meta,
		workload: wl,
		node:     opts.Node,
		store:    opts.Store,
		out:      opts.Out,
		bufSize:  bufSize,
		done:     make(chan struct{}),
	}
}

// Start begins tailing the file from the last saved offset.
func (r *Reader) Start() {
	go r.run()
}

// Stop signals the reader to stop.
func (r *Reader) Stop() {
	r.closeOnce.Do(func() {
		close(r.done)
	})
}

func (r *Reader) run() {
	offset, err := r.store.LoadOffset(r.path)
	if err != nil {
		slog.Warn("failed to load offset, starting from 0", "path", r.path, "error", err)
		offset = 0
	}

	r.readFrom(offset)
}

// readFrom reads the file starting from the given offset and processes new lines.
func (r *Reader) readFrom(offset int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := os.Open(r.path)
	if err != nil {
		slog.Error("failed to open log file", "path", r.path, "error", err)
		return
	}
	defer f.Close()

	// Seek to saved offset
	if offset > 0 {
		fi, err := f.Stat()
		if err == nil && offset <= fi.Size() {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				slog.Warn("seek failed, reading from start", "path", r.path, "error", err)
			}
		} else {
			// File was truncated/rotated, read from start
			offset = 0
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, r.bufSize), 1024*1024) // max 1MB per line

	var partialMsg string

	for scanner.Scan() {
		select {
		case <-r.done:
			return
		default:
		}

		line := scanner.Text()
		criLine, ok := parser.ParseCRI(line)
		if !ok {
			continue
		}

		// Handle multiline / partial logs
		if criLine.IsPartial {
			partialMsg += criLine.Message
			continue
		}

		msg := partialMsg + criLine.Message
		partialMsg = ""

		level := parser.DetectLevel(msg)

		entry := model.LogEntry{
			Timestamp:    criLine.Timestamp,
			Date:         criLine.Timestamp.Format("2006-01-02"),
			Namespace:    r.meta.Namespace,
			Workload:     r.workload.Name,
			WorkloadType: r.workload.Type,
			Pod:          r.meta.Pod,
			Container:    r.meta.Container,
			Node:         r.node,
			Stream:       criLine.Stream,
			Level:        level,
			Message:      msg,
		}

		select {
		case r.out <- entry:
		case <-r.done:
			return
		}
	}

	// Save current offset
	pos, err := f.Seek(0, io.SeekCurrent)
	if err == nil {
		_ = r.store.SaveOffset(r.path, pos)
	}
}

// ReadNew reads only new data appended since the last read.
// Called when fsnotify fires a Write event.
func (r *Reader) ReadNew() {
	offset, err := r.store.LoadOffset(r.path)
	if err != nil {
		offset = 0
	}
	r.readFrom(offset)
}
