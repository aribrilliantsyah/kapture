package badger

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	badgeropts "github.com/dgraph-io/badger/v4/options"
	"github.com/ordinary/k8s-log-catcher/internal/model"
)

// Store implements storage.Store using BadgerDB.
type Store struct {
	db        *badgerdb.DB
	path      string
	retention time.Duration
	maxDisk   int64
	gcTicker  *time.Ticker
	done      chan struct{}
	seq       atomic.Uint32
	mu        sync.Mutex
}

// Options for creating a new BadgerDB store.
type Options struct {
	Path        string
	Retention   time.Duration
	MaxDisk     int64
	GCInterval  time.Duration
	Compression string
}

// New creates a new BadgerDB-backed store.
func New(opts Options) (*Store, error) {
	if err := os.MkdirAll(opts.Path, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	bopts := badgerdb.DefaultOptions(opts.Path).
		WithLogger(nil).
		WithNumVersionsToKeep(1).
		WithNumGoroutines(4).
		WithValueLogFileSize(64 << 20). // 64 MB
		WithNumMemtables(2).
		WithNumLevelZeroTables(2).
		WithNumLevelZeroTablesStall(4).
		WithBaseTableSize(4 << 20).    // 4 MB
		WithBaseLevelSize(16 << 20).   // 16 MB
		WithDetectConflicts(false)

	switch opts.Compression {
	case "zstd":
		bopts = bopts.WithCompression(badgeropts.ZSTD)
	case "none":
		bopts = bopts.WithCompression(badgeropts.None)
	default: // snappy
		bopts = bopts.WithCompression(badgeropts.Snappy)
	}

	db, err := badgerdb.Open(bopts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	gcInterval := opts.GCInterval
	if gcInterval == 0 {
		gcInterval = 5 * time.Minute
	}

	s := &Store{
		db:        db,
		path:      opts.Path,
		retention: opts.Retention,
		maxDisk:   opts.MaxDisk,
		gcTicker:  time.NewTicker(gcInterval),
		done:      make(chan struct{}),
	}

	go s.gcLoop()

	return s, nil
}

// Write stores a batch of log entries.
func (s *Store) Write(entries []model.LogEntry) error {
	wb := s.db.NewWriteBatch()
	defer wb.Cancel()

	ttl := s.retention

	for _, e := range entries {
		seq := s.seq.Add(1)
		date := dateFromTime(e.Timestamp)

		key := buildKey(date, e.Namespace, e.Workload, e.Pod, e.Container, e.Timestamp.UnixNano(), seq)

		val, err := json.Marshal(e)
		if err != nil {
			continue
		}

		entry := badgerdb.NewEntry(key, val)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}

		if err := wb.SetEntry(entry); err != nil {
			return fmt.Errorf("batch set: %w", err)
		}
	}

	return wb.Flush()
}

// Query retrieves log entries matching the request.
func (s *Store) Query(req model.QueryRequest) (*model.QueryResult, error) {
	result := &model.QueryResult{}
	limit := req.Limit
	if limit <= 0 || limit > 10000 {
		limit = 100
	}

	var searchRe *regexp.Regexp
	if req.Regex != "" {
		var err error
		searchRe, err = regexp.Compile(req.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	}

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.PrefetchSize = limit

		if req.Sort == "desc" {
			opts.Reverse = true
		}

		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := s.buildQueryPrefix(req)

		var seekKey []byte
		if req.Cursor != "" {
			seekKey = []byte(req.Cursor)
			if req.Sort == "desc" {
				seekKey = append(seekKey, 0x00)
			}
		} else if req.Sort == "desc" {
			// Seek to end of prefix range
			seekKey = append(prefix, 0xFF)
		} else {
			seekKey = prefix
		}

		count := 0
		it.Seek(seekKey)

		// Skip cursor key itself
		if req.Cursor != "" && it.Valid() {
			key := it.Item().Key()
			if string(key) == req.Cursor {
				it.Next()
			}
		}

		for ; it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Check prefix
			if !hasPrefix(key, prefix) {
				break
			}

			// Skip offset/meta keys
			if key[0] == '_' {
				continue
			}

			err := item.Value(func(val []byte) error {
				var entry model.LogEntry
				if err := json.Unmarshal(val, &entry); err != nil {
					return nil // skip malformed
				}

				// Apply filters
				if !s.matchesFilter(entry, req, searchRe) {
					return nil
				}

				result.Entries = append(result.Entries, entry)
				count++
				return nil
			})
			if err != nil {
				return err
			}

			if count >= limit {
				// Set next cursor
				result.NextCursor = string(item.Key())
				break
			}
		}

		return nil
	})

	return result, err
}

// buildQueryPrefix determines the best key prefix for a query.
func (s *Store) buildQueryPrefix(req model.QueryRequest) []byte {
	if req.Date != "" && req.Namespace != "" && req.Workload != "" && req.Pod != "" {
		return buildPodPrefix(req.Date, req.Namespace, req.Workload, req.Pod)
	}
	if req.Date != "" && req.Namespace != "" && req.Workload != "" {
		return buildWorkloadPrefix(req.Date, req.Namespace, req.Workload)
	}
	if req.Date != "" && req.Namespace != "" {
		return buildNSPrefix(req.Date, req.Namespace)
	}
	if req.Date != "" {
		return buildDatePrefix(req.Date)
	}
	// No date filter — scan everything (with time-based filter in matchesFilter)
	return nil
}

// matchesFilter checks if a log entry matches the query filters.
func (s *Store) matchesFilter(entry model.LogEntry, req model.QueryRequest, searchRe *regexp.Regexp) bool {
	if req.From != nil && entry.Timestamp.Before(*req.From) {
		return false
	}
	if req.To != nil && entry.Timestamp.After(*req.To) {
		return false
	}
	if req.Namespace != "" && entry.Namespace != req.Namespace {
		return false
	}
	if req.Workload != "" && entry.Workload != req.Workload {
		return false
	}
	if req.WorkloadType != "" && entry.WorkloadType != req.WorkloadType {
		return false
	}
	if req.Pod != "" && entry.Pod != req.Pod {
		return false
	}
	if req.Container != "" && entry.Container != req.Container {
		return false
	}
	if req.Level != "" {
		levels := strings.Split(req.Level, ",")
		matched := false
		for _, l := range levels {
			if strings.EqualFold(entry.Level, strings.TrimSpace(l)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, excl := range req.ExcludeNS {
		if entry.Namespace == excl {
			return false
		}
	}
	if req.Search != "" && !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(req.Search)) {
		return false
	}
	if searchRe != nil && !searchRe.MatchString(entry.Message) {
		return false
	}
	return true
}

// Dates returns all unique dates that have log data.
func (s *Store) Dates() ([]string, error) {
	dates := make(map[string]bool)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if key[0] == '_' {
				continue
			}
			idx := strings.IndexByte(key, ':')
			if idx > 0 {
				date := key[:idx]
				if len(date) == 10 { // YYYY-MM-DD
					if !dates[date] {
						dates[date] = true
					}
				}
			}
		}
		return nil
	})

	result := make([]string, 0, len(dates))
	for d := range dates {
		result = append(result, d)
	}
	return result, err
}

// Namespaces returns all unique namespaces.
func (s *Store) Namespaces() ([]string, error) {
	return s.uniqueField(1)
}

// Workloads returns all unique workloads, optionally filtered by namespace.
func (s *Store) Workloads(namespace string) ([]string, error) {
	workloads := make(map[string]bool)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if key[0] == '_' {
				continue
			}
			parts := strings.SplitN(key, keySep, 7)
			if len(parts) < 4 {
				continue
			}
			if namespace != "" && parts[1] != namespace {
				continue
			}
			workloads[parts[2]] = true
		}
		return nil
	})

	result := make([]string, 0, len(workloads))
	for w := range workloads {
		result = append(result, w)
	}
	return result, err
}

// Pods returns unique pods, optionally filtered by namespace and workload.
func (s *Store) Pods(namespace, workload string) ([]string, error) {
	pods := make(map[string]bool)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if key[0] == '_' {
				continue
			}
			parts := strings.SplitN(key, keySep, 7)
			if len(parts) < 5 {
				continue
			}
			if namespace != "" && parts[1] != namespace {
				continue
			}
			if workload != "" && parts[2] != workload {
				continue
			}
			pods[parts[3]] = true
		}
		return nil
	})

	result := make([]string, 0, len(pods))
	for p := range pods {
		result = append(result, p)
	}
	return result, err
}

// uniqueField collects unique values at a given key part index.
func (s *Store) uniqueField(idx int) ([]string, error) {
	vals := make(map[string]bool)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if key[0] == '_' {
				continue
			}
			parts := strings.SplitN(key, keySep, idx+2)
			if len(parts) > idx {
				vals[parts[idx]] = true
			}
		}
		return nil
	})

	result := make([]string, 0, len(vals))
	for v := range vals {
		result = append(result, v)
	}
	return result, err
}

// Delete removes log entries matching the request.
func (s *Store) Delete(req model.DeleteRequest) (int64, error) {
	if req.All {
		return s.deleteAll()
	}

	var deleted int64

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		var keysToDelete [][]byte

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().KeyCopy(nil)
			if key[0] == '_' {
				continue
			}

			keyStr := string(key)
			parts := strings.SplitN(keyStr, keySep, 7)
			if len(parts) < 3 {
				continue
			}

			date := parts[0]
			ns := parts[1]
			wl := parts[2]

			if req.BeforeDate != "" && date >= req.BeforeDate {
				continue
			}
			if req.Namespace != "" && ns != req.Namespace {
				continue
			}
			if req.Workload != "" && wl != req.Workload {
				continue
			}

			keysToDelete = append(keysToDelete, key)
		}

		for _, k := range keysToDelete {
			if err := txn.Delete(k); err != nil {
				return err
			}
			deleted++
		}
		return nil
	})

	return deleted, err
}

// deleteAll drops the entire database and recreates it.
func (s *Store) deleteAll() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int64

	// Count entries first
	_ = s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			if it.Item().Key()[0] != '_' {
				count++
			}
		}
		return nil
	})

	return count, s.db.DropAll()
}

// ResetAll drops all log data.
func (s *Store) ResetAll() error {
	_, err := s.deleteAll()
	return err
}

// StorageInfo returns current storage usage statistics.
func (s *Store) StorageInfo() (*model.StorageInfo, error) {
	info := &model.StorageInfo{
		MaxBytes: s.maxDisk,
	}

	dateStats := make(map[string]*model.DateBreakdown)

	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			if key[0] == '_' {
				continue
			}

			info.EntryCount++
			estSize := int64(len(item.Key())) + item.ValueSize()
			info.UsedBytes += estSize

			idx := strings.IndexByte(key, ':')
			if idx > 0 {
				date := key[:idx]
				if len(date) == 10 {
					ds, ok := dateStats[date]
					if !ok {
						ds = &model.DateBreakdown{Date: date}
						dateStats[date] = ds
					}
					ds.EntryCount++
					ds.SizeBytes += estSize
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	info.Dates = make([]model.DateBreakdown, 0, len(dateStats))
	for _, ds := range dateStats {
		info.Dates = append(info.Dates, *ds)
		if info.OldestDate == "" || ds.Date < info.OldestDate {
			info.OldestDate = ds.Date
		}
		if ds.Date > info.NewestDate {
			info.NewestDate = ds.Date
		}
	}

	return info, nil
}

// SaveOffset persists the file read offset.
func (s *Store) SaveOffset(file string, offset int64) error {
	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Set(offsetKey(file), encodeOffset(offset))
	})
}

// LoadOffset retrieves the file read offset.
func (s *Store) LoadOffset(file string) (int64, error) {
	var offset int64
	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(offsetKey(file))
		if err == badgerdb.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			offset = decodeOffset(val)
			return nil
		})
	})
	return offset, err
}

// Close shuts down the storage.
func (s *Store) Close() error {
	close(s.done)
	s.gcTicker.Stop()
	return s.db.Close()
}

// gcLoop runs garbage collection periodically.
func (s *Store) gcLoop() {
	for {
		select {
		case <-s.done:
			return
		case <-s.gcTicker.C:
			s.runGC()
		}
	}
}

// runGC performs value log garbage collection.
func (s *Store) runGC() {
	for {
		err := s.db.RunValueLogGC(0.5)
		if err != nil {
			break
		}
	}

	// Check disk cap
	if s.maxDisk > 0 {
		info, err := s.StorageInfo()
		if err != nil {
			return
		}
		if info.UsedBytes > s.maxDisk {
			slog.Warn("storage exceeds disk cap, cleaning oldest data",
				"used", info.UsedBytes, "max", s.maxDisk)
			s.cleanOldest(info)
		}
	}
}

// cleanOldest removes the oldest date's data to free disk space.
func (s *Store) cleanOldest(info *model.StorageInfo) {
	if info.OldestDate == "" {
		return
	}
	deleted, err := s.Delete(model.DeleteRequest{BeforeDate: info.OldestDate + "~"}) // ~ > any date char
	if err != nil {
		slog.Error("failed to clean oldest data", "error", err)
		return
	}
	slog.Info("cleaned oldest data", "date", info.OldestDate, "deleted", deleted)
}

func hasPrefix(key, prefix []byte) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(key) < len(prefix) {
		return false
	}
	for i, b := range prefix {
		if key[i] != b {
			return false
		}
	}
	return true
}
