package tailer

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aribrilliantsyah/kapture/internal/agent/enricher"
	"github.com/aribrilliantsyah/kapture/internal/agent/parser"
	"github.com/aribrilliantsyah/kapture/internal/model"
	"github.com/aribrilliantsyah/kapture/internal/storage"
)

const (
	maxLineBytes     = 4 << 20   // drop a runaway line without newline past this
	maxMessageBytes  = 256 << 10 // cap for joined partial / multiline messages
	multilineWindow  = 2 * time.Second
	defaultReadBytes = 64 << 10
)

// Reader follows one container log file. It keeps the file open so a rotated
// file is read to the end before switching to the new one, and it only
// advances its offset past complete lines.
type Reader struct {
	path     string
	meta     enricher.FileMeta
	workload enricher.Workload
	node     string
	store    storage.Store
	out      chan<- model.LogEntry
	limiter  *rateLimiter
	done     <-chan struct{}
	bufSize  int
	resolver *enricher.Resolver
	resolved bool

	mu          sync.Mutex
	f           *os.File
	br          *bufio.Reader
	inode       uint64
	offset      int64  // start of the next unread line
	savedOffset int64  // last offset persisted to the store
	carry       []byte // bytes of a line whose newline has not been written yet
	partial     *partialLine
	pending     *model.LogEntry // held back so continuation lines can join it
	openErr     bool
}

// partialLine accumulates CRI "P" fragments of one long line.
type partialLine struct {
	ts     time.Time
	stream string
	seq    int64
	msg    strings.Builder
}

// ReaderOpts configures a file reader.
type ReaderOpts struct {
	Path     string
	Meta     enricher.FileMeta
	Node     string
	Store    storage.Store
	Out      chan<- model.LogEntry
	Limiter  *rateLimiter
	Done     <-chan struct{}
	BufSize  int
	Resolver *enricher.Resolver
}

// NewReader creates a file reader. Nothing is read until Poll.
func NewReader(opts ReaderOpts) *Reader {
	bufSize := opts.BufSize
	if bufSize < defaultReadBytes {
		bufSize = defaultReadBytes
	}
	return &Reader{
		path:     opts.Path,
		meta:     opts.Meta,
		workload: enricher.ExtractWorkload(opts.Meta.Pod),
		resolver: opts.Resolver,
		node:     opts.Node,
		store:    opts.Store,
		out:      opts.Out,
		limiter:  opts.Limiter,
		done:     opts.Done,
		bufSize:  bufSize,
	}
}

// Poll reads everything appended since the last call and follows rotation
// and truncation.
func (r *Reader) Poll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.resolved {
		// Asked once per file, from the poll loop rather than under the tailer lock.
		r.workload = r.resolver.Resolve(r.meta.Namespace, r.meta.Pod)
		r.resolved = true
	}
	if r.f == nil && !r.open(false) {
		return
	}
	r.drain()

	fi, err := os.Stat(r.path)
	if err != nil {
		return // gone; the tailer removes the reader
	}
	switch {
	case inodeOf(fi) != r.inode:
		// kubelet rotated the file: the old one is drained, start the new one.
		r.closeFile()
		if r.open(true) {
			r.drain()
		}
	case fi.Size() < r.offset:
		// truncated in place (copytruncate)
		if _, err := r.f.Seek(0, io.SeekStart); err == nil {
			r.br.Reset(r.f)
			r.offset, r.carry = 0, nil
			r.drain()
		}
	}
	r.saveOffset()
}

// Close drains what is left, then releases the file.
func (r *Reader) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f != nil {
		r.drain()
		r.saveOffset()
	}
	r.closeFile()
}

// open opens the file at its saved offset, or at 0 when fresh is set or the
// saved position belongs to an older (rotated or truncated) file.
func (r *Reader) open(fresh bool) bool {
	f, err := os.Open(r.path)
	if err != nil {
		if !r.openErr {
			slog.Warn("cannot open log file (is its symlink target mounted?)", "path", r.path, "error", err)
			r.openErr = true
		}
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return false
	}
	r.openErr = false
	ino := inodeOf(fi)

	var offset int64
	if !fresh {
		saved, savedIno, err := r.store.LoadOffset(r.path)
		if err == nil && (savedIno == 0 || savedIno == ino) && saved <= fi.Size() {
			offset = saved
		}
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		offset = 0
	}
	r.f, r.inode, r.offset, r.carry = f, ino, offset, nil
	r.br = bufio.NewReaderSize(f, r.bufSize)
	r.savedOffset = -1 // persist the (possibly new) inode on the next save
	return true
}

func (r *Reader) closeFile() {
	r.flushPending()
	if r.f != nil {
		r.f.Close()
		r.f, r.br = nil, nil
	}
}

func (r *Reader) drain() {
	for {
		select {
		case <-r.done:
			return
		default:
		}
		chunk, err := r.br.ReadBytes('\n')
		if err != nil {
			// No newline yet: keep the bytes, but don't move the offset past them.
			r.carry = append(r.carry, chunk...)
			if len(r.carry) > maxLineBytes {
				slog.Warn("dropping oversized log line", "path", r.path, "bytes", len(r.carry))
				r.offset += int64(len(r.carry))
				r.carry = nil
			}
			break
		}
		line := chunk
		if len(r.carry) > 0 {
			line = append(r.carry, chunk...)
			r.carry = nil
		}
		start := r.offset
		r.offset += int64(len(line))
		r.handle(line, start)
	}
	r.flushPending()
}

func (r *Reader) handle(raw []byte, start int64) {
	cl, ok := parser.ParseLine(strings.TrimRight(string(raw), "\r\n"))
	if !ok {
		return
	}
	if cl.IsPartial {
		if r.partial == nil {
			r.partial = &partialLine{ts: cl.Timestamp, stream: cl.Stream, seq: start}
		}
		if r.partial.msg.Len() < maxMessageBytes {
			r.partial.msg.WriteString(cl.Message)
		}
		return
	}
	ts, msg, seq := cl.Timestamp, cl.Message, start
	if p := r.partial; p != nil {
		p.msg.WriteString(msg)
		ts, msg, seq = p.ts, p.msg.String(), p.seq
		r.partial = nil
	}
	r.emit(ts, cl.Stream, msg, seq)
}

// emit turns a complete line into an entry. Indented lines that follow an
// entry closely (stack traces) are appended to it instead.
func (r *Reader) emit(ts time.Time, stream, msg string, seq int64) {
	if p := r.pending; p != nil && p.Stream == stream && isContinuation(msg) &&
		ts.Sub(p.Timestamp) < multilineWindow && len(p.Message) < maxMessageBytes {
		p.Message += "\n" + msg
		return
	}
	r.flushPending()
	r.pending = &model.LogEntry{
		Timestamp:    ts,
		Date:         ts.UTC().Format("2006-01-02"),
		Namespace:    r.meta.Namespace,
		Workload:     r.workload.Name,
		WorkloadType: r.workload.Type,
		Pod:          r.meta.Pod,
		Container:    r.meta.Container,
		Node:         r.node,
		Stream:       stream,
		Message:      msg,
		Seq:          uint64(seq) + 1,
	}
}

func (r *Reader) flushPending() {
	e := r.pending
	if e == nil {
		return
	}
	r.pending = nil
	e.Level = parser.DetectLevel(e.Message)
	r.limiter.take(r.done)
	select {
	case r.out <- *e:
	case <-r.done:
	}
}

func (r *Reader) saveOffset() {
	if r.offset == r.savedOffset {
		return
	}
	if err := r.store.SaveOffset(r.path, r.offset, r.inode); err != nil {
		slog.Warn("failed to save offset", "path", r.path, "error", err)
		return
	}
	r.savedOffset = r.offset
}

func isContinuation(msg string) bool {
	return strings.HasPrefix(msg, " ") || strings.HasPrefix(msg, "\t") ||
		strings.HasPrefix(msg, "Caused by:") || strings.HasPrefix(msg, "...")
}
