package badger

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	badgeropts "github.com/dgraph-io/badger/v4/options"
	"github.com/aribrilliantsyah/kapture/internal/model"
)

const indexFlushInterval = 5 * time.Second

// Store implements storage.Store using BadgerDB.
type Store struct {
	db   *badgerdb.DB
	opts Options
	seq  atomic.Uint64
	idx  *index

	// Writes take the read lock; DropPrefix takes the write lock so writes wait
	// for it instead of failing with ErrBlockedWrites.
	mu sync.RWMutex

	done chan struct{}
	wg   sync.WaitGroup
}

// Options for creating a new BadgerDB store.
type Options struct {
	Path        string
	Node        string         // node name reported with every entry
	Location    *time.Location // days (keys, rollups, deletes) follow this zone; nil = UTC
	Retention   time.Duration  // 0 keeps logs until max_disk forces the oldest day out
	MaxDisk     int64
	GCInterval  time.Duration
	Compression string
}

// New opens (or creates) a BadgerDB-backed store.
func New(opts Options) (*Store, error) {
	if err := os.MkdirAll(opts.Path, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	bopts := badgerdb.DefaultOptions(opts.Path).
		WithLogger(nil).
		WithNumVersionsToKeep(1).
		WithNumGoroutines(4).
		WithValueLogFileSize(64 << 20).
		WithNumMemtables(2).
		WithMemTableSize(16 << 20).
		WithNumLevelZeroTables(2).
		WithNumLevelZeroTablesStall(4).
		WithBaseTableSize(4 << 20).
		WithBaseLevelSize(16 << 20).
		WithBlockCacheSize(16 << 20).
		WithIndexCacheSize(8 << 20).
		WithDetectConflicts(false)

	switch opts.Compression {
	case "zstd":
		bopts = bopts.WithCompression(badgeropts.ZSTD)
	case "none":
		bopts = bopts.WithCompression(badgeropts.None)
	default:
		bopts = bopts.WithCompression(badgeropts.Snappy)
	}

	db, err := badgerdb.Open(bopts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}
	if opts.GCInterval <= 0 {
		opts.GCInterval = 5 * time.Minute
	}
	if opts.Location == nil {
		opts.Location = time.UTC
	}

	s := &Store{db: db, opts: opts, idx: newIndex(opts.Location), done: make(chan struct{})}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate storage: %w", err)
	}
	if err := s.idx.load(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("load index: %w", err)
	}
	s.wg.Add(1)
	go s.loop()
	return s, nil
}

// migrate upgrades older stores:
//   - schema 2 (time-ordered keys, no index): build the index from the keys.
//   - time zone changed (schemas before 4 always used UTC days): move every
//     log to the day of its timestamp in the new zone, then rebuild the index.
//   - anything older: clear logs and read offsets so the agent re-ingests
//     whatever container logs are still on disk.
func (s *Store) migrate() error {
	version, err := s.meta(schemaKey)
	if err != nil {
		return err
	}
	stored, err := s.meta(tzKey)
	if err != nil {
		return err
	}
	data := s.hasData()
	switch version {
	case schemaVersion, "3", "2":
	default:
		if data {
			slog.Warn("old storage layout found: clearing stored logs and offsets, container logs on disk will be re-ingested")
			if err := s.db.DropAll(); err != nil {
				return err
			}
			data = false
		}
	}
	if stored == "" {
		stored = "UTC"
	}
	reindex := version == "2"
	if data && stored != s.opts.Location.String() {
		slog.Info("time zone changed, moving stored logs to local days (one-time)", "from", stored, "to", s.opts.Location.String())
		if err := s.rekey(); err != nil {
			return fmt.Errorf("re-date logs: %w", err)
		}
		reindex = true
	}
	if reindex && data {
		slog.Info("building workload index from stored logs (one-time)")
		if err := s.rebuildIndex(); err != nil {
			return err
		}
	}
	return s.db.Update(func(txn *badgerdb.Txn) error {
		if err := txn.Set([]byte(schemaKey), []byte(schemaVersion)); err != nil {
			return err
		}
		return txn.Set([]byte(tzKey), []byte(s.opts.Location.String()))
	})
}

func (s *Store) meta(key string) (string, error) {
	var v string
	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(key))
		if errors.Is(err, badgerdb.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		b, err := item.ValueCopy(nil)
		v = string(b)
		return err
	})
	return v, err
}

// rekey moves every log key to the day of its timestamp in the store's zone.
// Values and expiry are kept; the read snapshot does not see the new keys.
func (s *Store) rekey() error {
	wb := s.db.NewWriteBatch()
	defer wb.Cancel()
	moved := 0
	err := s.db.View(func(txn *badgerdb.Txn) error {
		it := txn.NewIterator(badgerdb.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(dataLo); it.Valid(); it.Next() {
			item := it.Item()
			if bytes.Compare(item.Key(), dataHi) >= 0 {
				break
			}
			k, ok := parseKey(item.Key())
			if !ok {
				continue
			}
			date := dateOf(k.ts, s.opts.Location)
			if date == k.date {
				continue
			}
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			old := item.KeyCopy(nil)
			k.date = date
			e := badgerdb.NewEntry(encodeKey(k), val)
			e.ExpiresAt = item.ExpiresAt()
			if err := wb.SetEntry(e); err != nil {
				return err
			}
			if err := wb.Delete(old); err != nil {
				return err
			}
			moved++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := wb.Flush(); err != nil {
		return err
	}
	slog.Info("stored logs re-dated", "moved", moved)
	return nil
}

// rebuildIndex recomputes rollups and the pod catalog from the log keys.
func (s *Store) rebuildIndex() error {
	if err := s.db.DropPrefix([]byte(aggPrefix), []byte(catPrefix)); err != nil {
		return err
	}
	idx := newIndex(s.opts.Location)
	_, _, err := s.scan(dataLo, dataHi, false, false, func(k keyInfo, item *badgerdb.Item) (bool, error) {
		idx.observe(k, item.EstimatedSize())
		return true, nil
	})
	if err != nil {
		return err
	}
	return idx.flush(s.db)
}

func (s *Store) hasData() bool {
	found := false
	_ = s.db.View(func(txn *badgerdb.Txn) error {
		it := txn.NewIterator(keysOnly())
		defer it.Close()
		it.Seek(dataLo)
		found = it.Valid() && bytes.Compare(it.Item().Key(), dataHi) < 0
		return nil
	})
	return found
}

// Write stores a batch of log entries. Entries already past retention are dropped.
func (s *Store) Write(entries []model.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	wb := s.db.NewWriteBatch()
	defer wb.Cancel()

	type written struct {
		k    keyInfo
		size int64
	}
	now := time.Now()
	done := make([]written, 0, len(entries))
	for _, e := range entries {
		var expires time.Time
		if s.opts.Retention > 0 {
			if expires = e.Timestamp.Add(s.opts.Retention); expires.Before(now) {
				continue
			}
		}
		k := keyInfo{
			ts:        e.Timestamp.UnixNano(),
			ns:        clean(e.Namespace),
			wl:        clean(e.Workload),
			wtype:     clean(e.WorkloadType),
			pod:       clean(e.Pod),
			container: clean(e.Container),
			level:     model.NormalizeLevel(e.Level),
			seq:       e.Seq,
		}
		k.date = dateOf(k.ts, s.opts.Location)
		if k.seq == 0 {
			k.seq = 1<<63 | s.seq.Add(1)
		}
		key, val := encodeKey(k), encodeValue(e.Stream, e.Message)
		entry := badgerdb.NewEntry(key, val)
		if !expires.IsZero() {
			entry.ExpiresAt = uint64(expires.Unix())
		}
		if err := wb.SetEntry(entry); err != nil {
			return fmt.Errorf("batch set: %w", err)
		}
		done = append(done, written{k, int64(len(key) + len(val))})
	}
	if err := wb.Flush(); err != nil {
		return err
	}
	s.idx.mu.Lock()
	for _, w := range done {
		s.idx.observeLocked(w.k, w.size)
	}
	s.idx.mu.Unlock()
	return nil
}

// Dates returns every date that has log data, oldest first.
func (s *Store) Dates() ([]string, error) {
	stats := s.idx.dateStats()
	out := make([]string, len(stats))
	for i, d := range stats {
		out[len(stats)-1-i] = d.Date
	}
	return out, nil
}

// Catalog lists every container with stored logs, including replaced pods.
func (s *Store) Catalog() []model.CatalogItem {
	items := s.idx.catalog()
	for i := range items {
		items[i].Node = s.opts.Node
	}
	return items
}

// Recap returns daily per-workload rollups between two dates (inclusive).
func (s *Store) Recap(from, to, namespace string) []model.WorkloadDay {
	return s.idx.recap(from, to, namespace)
}

// Namespaces returns all namespaces with logs.
func (s *Store) Namespaces() ([]string, error) {
	return s.idx.distinct(func(c *model.CatalogItem) string { return c.Namespace }), nil
}

// Workloads returns workloads, optionally within one namespace.
func (s *Store) Workloads(namespace string) ([]string, error) {
	return s.idx.distinct(func(c *model.CatalogItem) string {
		if namespace != "" && c.Namespace != namespace {
			return ""
		}
		return c.Workload
	}), nil
}

// Pods returns pods, optionally filtered by namespace and workload.
func (s *Store) Pods(namespace, workload string) ([]string, error) {
	return s.idx.distinct(func(c *model.CatalogItem) string {
		if (namespace != "" && c.Namespace != namespace) || (workload != "" && c.Workload != workload) {
			return ""
		}
		return c.Pod
	}), nil
}

// Delete removes log entries matching the request.
func (s *Store) Delete(req model.DeleteRequest) (int64, error) {
	if req.All {
		return s.dropAll()
	}
	if req.Date == "" && req.BeforeDate == "" && req.Namespace == "" && req.Workload == "" {
		return 0, errors.New("nothing to delete: set date, before, namespace or workload")
	}
	inDates := func(d string) bool {
		return (req.Date == "" && req.BeforeDate == "") || d == req.Date || (req.BeforeDate != "" && d < req.BeforeDate)
	}
	if req.Namespace == "" && req.Workload == "" {
		var targets []string
		dates, _ := s.Dates()
		for _, d := range dates {
			if inDates(d) {
				targets = append(targets, d)
			}
		}
		return s.dropDates(targets)
	}

	n, err := s.deleteMatching(req)
	if err != nil {
		return n, err
	}
	s.idx.removeDays(func(k aggKey) bool {
		return (req.Namespace == "" || k.ns == req.Namespace) && (req.Workload == "" || k.wl == req.Workload) && inDates(k.date)
	})
	s.flushIndex()
	return n, nil
}

// deleteMatching deletes entries of a namespace/workload, optionally limited by date.
func (s *Store) deleteMatching(req model.DeleteRequest) (int64, error) {
	lo, hi := dataLo, dataHi
	if req.Date != "" {
		lo, hi = []byte(req.Date+":"), []byte(req.Date+";")
	} else if req.BeforeDate != "" {
		hi = []byte(req.BeforeDate)
	}

	wb := s.db.NewWriteBatch()
	defer wb.Cancel()
	var deleted int64
	_, _, err := s.scan(lo, hi, false, false, func(k keyInfo, item *badgerdb.Item) (bool, error) {
		if (req.Namespace != "" && k.ns != req.Namespace) || (req.Workload != "" && k.wl != req.Workload) {
			return true, nil
		}
		if err := wb.Delete(item.KeyCopy(nil)); err != nil {
			return false, err
		}
		deleted++
		return true, nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, wb.Flush()
}

// dropDates removes whole days, which Badger does without rewriting data.
func (s *Store) dropDates(dates []string) (int64, error) {
	if len(dates) == 0 {
		return 0, nil
	}
	set := map[string]bool{}
	prefixes := make([][]byte, len(dates))
	for i, d := range dates {
		set[d] = true
		prefixes[i] = []byte(d + ":")
	}
	var count int64
	for _, st := range s.idx.dateStats() {
		if set[st.Date] {
			count += st.EntryCount
		}
	}
	s.mu.Lock()
	err := s.db.DropPrefix(prefixes...)
	s.mu.Unlock()
	if err != nil {
		return 0, err
	}
	s.idx.removeDays(func(k aggKey) bool { return set[k.date] })
	s.flushIndex()
	return count, nil
}

// dropAll removes every log and the index but keeps read offsets, so lines
// already collected are not ingested again.
func (s *Store) dropAll() (int64, error) {
	var count int64
	for _, st := range s.idx.dateStats() {
		count += st.EntryCount
	}
	prefixes := [][]byte{[]byte(aggPrefix), []byte(catPrefix)}
	for c := byte('0'); c <= '9'; c++ {
		prefixes = append(prefixes, []byte{c})
	}
	s.mu.Lock()
	err := s.db.DropPrefix(prefixes...)
	if err == nil {
		s.idx.reset()
	}
	s.mu.Unlock()
	return count, err
}

// StorageInfo returns current storage usage statistics.
func (s *Store) StorageInfo() (*model.StorageInfo, error) {
	info := &model.StorageInfo{Node: s.opts.Node, MaxBytes: s.opts.MaxDisk, Retention: "unlimited", Timezone: s.opts.Location.String()}
	if s.opts.Retention > 0 {
		info.Retention = s.opts.Retention.String()
	}
	info.Dates = s.idx.dateStats()
	var estimated int64
	for _, d := range info.Dates {
		info.EntryCount += d.EntryCount
		estimated += d.SizeBytes
	}
	if n := len(info.Dates); n > 0 {
		info.NewestDate, info.OldestDate = info.Dates[0].Date, info.Dates[n-1].Date
	}
	// Badger refreshes Size() only about once a minute.
	lsm, vlog := s.db.Size()
	info.UsedBytes = max(lsm+vlog, estimated)
	return info, nil
}

// SaveOffset persists the read position of a log file.
func (s *Store) SaveOffset(file string, offset int64, inode uint64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Set(offsetKey(file), encodeOffset(offset, inode))
	})
}

// LoadOffset returns the saved read position and inode of a log file.
func (s *Store) LoadOffset(file string) (int64, uint64, error) {
	var offset int64
	var inode uint64
	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(offsetKey(file))
		if errors.Is(err, badgerdb.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			offset, inode = decodeOffset(val)
			return nil
		})
	})
	return offset, inode, err
}

// DeleteOffset forgets a log file that no longer exists.
func (s *Store) DeleteOffset(file string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Delete(offsetKey(file))
	})
}

// Close flushes the index and shuts down the storage.
func (s *Store) Close() error {
	close(s.done)
	s.wg.Wait()
	s.flushIndex()
	return s.db.Close()
}

func (s *Store) loop() {
	defer s.wg.Done()
	flush := time.NewTicker(indexFlushInterval)
	defer flush.Stop()
	gc := time.NewTicker(s.opts.GCInterval)
	defer gc.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-flush.C:
			s.flushIndex()
		case <-gc.C:
			s.runGC()
		}
	}
}

func (s *Store) flushIndex() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.idx.flush(s.db); err != nil {
		slog.Warn("failed to persist index", "error", err)
	}
}

func (s *Store) runGC() {
	for s.db.RunValueLogGC(0.5) == nil {
	}
	s.enforceRetention()
	s.enforceDiskCap()
}

// enforceRetention drops whole days that are entirely past retention.
func (s *Store) enforceRetention() {
	if s.opts.Retention <= 0 {
		return
	}
	cutoff := dateOf(time.Now().Add(-s.opts.Retention).UnixNano(), s.opts.Location)
	dates, _ := s.Dates()
	var old []string
	for _, d := range dates {
		if d < cutoff {
			old = append(old, d)
		}
	}
	if len(old) == 0 {
		return
	}
	n, err := s.dropDates(old)
	if err != nil {
		slog.Error("retention cleanup failed", "error", err)
		return
	}
	slog.Info("retention cleanup", "dates", old, "deleted", n)
}

// enforceDiskCap drops the oldest day while usage is above max_disk.
func (s *Store) enforceDiskCap() {
	if s.opts.MaxDisk <= 0 {
		return
	}
	lsm, vlog := s.db.Size()
	if lsm+vlog <= s.opts.MaxDisk {
		return
	}
	dates, _ := s.Dates()
	if len(dates) < 2 {
		slog.Warn("storage above disk cap but only one day of logs is left", "used", lsm+vlog, "max", s.opts.MaxDisk)
		return
	}
	n, err := s.dropDates(dates[:1])
	if err != nil {
		slog.Error("disk cap cleanup failed", "error", err)
		return
	}
	slog.Warn("storage above disk cap, dropped oldest day", "date", dates[0], "deleted", n, "used", lsm+vlog, "max", s.opts.MaxDisk)
}

func keysOnly() badgerdb.IteratorOptions {
	opts := badgerdb.DefaultIteratorOptions
	opts.PrefetchValues = false
	return opts
}
