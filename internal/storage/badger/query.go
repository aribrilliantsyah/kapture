package badger

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	"github.com/aribrilliantsyah/kapture/internal/logfilter"
	"github.com/aribrilliantsyah/kapture/internal/model"
)

// Scan budget per request, so an unselective search cannot pin the agent.
// A request that runs out returns what it found plus a cursor to continue.
const (
	scanMaxKeys = 3_000_000
	scanMaxTime = 5 * time.Second
)

// Query returns entries matching req in time order (newest first unless sort=asc).
func (s *Store) Query(req model.QueryRequest) (*model.QueryResult, error) {
	f, err := logfilter.New(req)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	limit = min(limit, 10000)

	res := &model.QueryResult{Entries: make([]model.LogEntry, 0, min(limit, 256))}
	lo, hi := keyRange(req, s.opts.Location)
	partial, last, err := s.scan(lo, hi, req.Desc(), true, func(k keyInfo, item *badgerdb.Item) (bool, error) {
		if !f.MatchMeta(k.ns, k.wl, k.wtype, k.pod, k.container, k.level) {
			return true, nil
		}
		matched := false
		err := item.Value(func(v []byte) error {
			stream, msg := decodeValue(v)
			if f.MatchText(msg) {
				res.Entries = append(res.Entries, s.toEntry(k, stream, msg))
				matched = true
			}
			return nil
		})
		if err != nil {
			return false, err
		}
		if matched && len(res.Entries) >= limit {
			res.NextCursor = strconv.FormatInt(k.ts, 10)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if partial {
		res.Partial = true
		res.NextCursor = strconv.FormatInt(last, 10)
	}
	return res, nil
}

// Volume counts matching entries per level in equal time buckets between
// req.From and req.To (default: the last 24h).
func (s *Store) Volume(req model.QueryRequest) (*model.VolumeResult, error) {
	f, err := logfilter.New(req)
	if err != nil {
		return nil, err
	}
	to := time.Now()
	if req.To != nil {
		to = *req.To
	}
	from := to.Add(-24 * time.Hour)
	if req.From != nil {
		from = *req.From
	}
	if !from.Before(to) {
		return nil, fmt.Errorf("from must be before to")
	}
	n := req.Buckets
	if n <= 0 {
		n = 60
	}
	n = min(n, 720)
	span := to.UnixNano() - from.UnixNano()
	width := (span + int64(n) - 1) / int64(n)

	res := &model.VolumeResult{
		From: from.UnixNano(), To: to.UnixNano(), BucketNanos: width,
		Buckets: make([]model.VolumeBucket, n), Totals: map[string]int64{},
	}
	for i := range res.Buckets {
		res.Buckets[i] = model.VolumeBucket{Start: res.From + int64(i)*width, Counts: map[string]int64{}}
	}

	r := req
	r.Date, r.Cursor, r.From, r.To = "", "", &from, &to
	lo, hi := keyRange(r, s.opts.Location)
	errs := map[[2]string]int64{}
	partial, _, err := s.scan(lo, hi, false, true, func(k keyInfo, item *badgerdb.Item) (bool, error) {
		if !f.MatchMeta(k.ns, k.wl, k.wtype, k.pod, k.container, k.level) {
			return true, nil
		}
		if f.NeedText() {
			ok := false
			if err := item.Value(func(v []byte) error {
				_, msg := decodeValue(v)
				ok = f.MatchText(msg)
				return nil
			}); err != nil {
				return false, err
			}
			if !ok {
				return true, nil
			}
		}
		idx := min(int((k.ts-res.From)/width), n-1)
		res.Buckets[idx].Counts[k.level]++
		res.Totals[k.level]++
		if k.level == "ERROR" || k.level == "FATAL" {
			errs[[2]string{k.ns, k.wl}]++
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	res.Partial = partial
	res.TopErrors = topWorkloads(errs, 10)
	return res, nil
}

func topWorkloads(m map[[2]string]int64, n int) []model.WorkloadCount {
	out := make([]model.WorkloadCount, 0, len(m))
	for k, c := range m {
		out = append(out, model.WorkloadCount{Namespace: k[0], Workload: k[1], Count: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func (s *Store) toEntry(k keyInfo, stream, msg string) model.LogEntry {
	return model.LogEntry{
		Timestamp:    time.Unix(0, k.ts).UTC(),
		Date:         k.date,
		Namespace:    k.ns,
		Workload:     k.wl,
		WorkloadType: k.wtype,
		Pod:          k.pod,
		Container:    k.container,
		Node:         s.opts.Node,
		Stream:       stream,
		Level:        k.level,
		Message:      msg,
		Seq:          k.seq,
	}
}

// keyRange turns the time filters of req into a [lo, hi) key range.
func keyRange(req model.QueryRequest, loc *time.Location) (lo, hi []byte) {
	lo, hi = dataLo, dataHi
	raise := func(b []byte) {
		if bytes.Compare(b, lo) > 0 {
			lo = b
		}
	}
	lower := func(b []byte) {
		if bytes.Compare(b, hi) < 0 {
			hi = b
		}
	}
	if req.Date != "" {
		raise([]byte(req.Date + ":"))
		lower([]byte(req.Date + ";"))
	}
	if req.From != nil {
		raise(tsBound(req.From.UnixNano(), loc))
	}
	if req.To != nil {
		lower(tsBound(req.To.UnixNano()+1, loc))
	}
	if n, err := strconv.ParseInt(req.Cursor, 10, 64); err == nil {
		if req.Desc() {
			lower(tsBound(n, loc))
		} else {
			raise(tsBound(n+1, loc))
		}
	}
	return lo, hi
}

// scan visits log keys in [lo, hi), newest first when desc, calling fn until it
// returns false. With budget set it gives up after scanMaxKeys/scanMaxTime and
// reports partial=true plus the timestamp of the last key visited.
func (s *Store) scan(lo, hi []byte, desc, budget bool, fn func(keyInfo, *badgerdb.Item) (bool, error)) (partial bool, last int64, err error) {
	if bytes.Compare(lo, hi) >= 0 {
		return false, 0, nil
	}
	deadline := time.Now().Add(scanMaxTime)
	err = s.db.View(func(txn *badgerdb.Txn) error {
		opts := keysOnly()
		opts.Reverse = desc
		it := txn.NewIterator(opts)
		defer it.Close()

		if desc {
			it.Seek(hi)
		} else {
			it.Seek(lo)
		}
		for n := 1; it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			if desc {
				if bytes.Compare(key, lo) < 0 {
					break
				}
				if bytes.Compare(key, hi) >= 0 {
					continue
				}
			} else if bytes.Compare(key, hi) >= 0 {
				break
			}
			k, ok := parseKey(key)
			if !ok {
				continue
			}
			last = k.ts
			more, err := fn(k, item)
			if err != nil || !more {
				return err
			}
			if n++; budget && n%4096 == 0 && (n >= scanMaxKeys || time.Now().After(deadline)) {
				partial = true
				return nil
			}
		}
		return nil
	})
	return partial, last, err
}

// scanKeys visits every raw key in [lo, hi) without parsing it.
func (s *Store) scanKeys(lo, hi []byte, fn func([]byte, *badgerdb.Item)) error {
	return s.db.View(func(txn *badgerdb.Txn) error {
		it := txn.NewIterator(keysOnly())
		defer it.Close()
		for it.Seek(lo); it.Valid(); it.Next() {
			item := it.Item()
			if bytes.Compare(item.Key(), hi) >= 0 {
				break
			}
			fn(item.Key(), item)
		}
		return nil
	})
}
