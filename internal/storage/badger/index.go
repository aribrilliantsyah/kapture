package badger

import (
	"encoding/json"
	"slices"
	"sort"
	"sync"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	"github.com/aribrilliantsyah/kapture/internal/model"
)

// The index keeps, next to the log lines themselves:
//
//	_agg:<date>:<ns>:<workload>        daily rollup (lines per level, bytes, pods seen)
//	_cat:<ns>:<pod>:<container>        every container ever seen (first/last line)
//
// It lives in memory, is persisted every few seconds, and answers dashboards,
// storage stats and pod history without scanning log lines. Pods of a
// deployment get new names on every rollout; the catalog keeps the old ones,
// grouped under the same workload.
const (
	aggPrefix     = "_agg:"
	catPrefix     = "_cat:"
	maxPodsPerDay = 1000
)

type aggKey struct{ date, ns, wl string }
type catKey struct{ ns, pod, container string }

type index struct {
	loc       *time.Location
	mu        sync.Mutex
	days      map[aggKey]*model.WorkloadDay
	cat       map[catKey]*model.CatalogItem
	dirtyDays map[aggKey]bool
	dirtyCat  map[catKey]bool
	goneDays  map[aggKey]bool
	goneCat   map[catKey]bool
}

func newIndex(loc *time.Location) *index {
	x := &index{loc: loc}
	x.resetLocked()
	return x
}

func (x *index) resetLocked() {
	x.days = map[aggKey]*model.WorkloadDay{}
	x.cat = map[catKey]*model.CatalogItem{}
	x.dirtyDays = map[aggKey]bool{}
	x.dirtyCat = map[catKey]bool{}
	x.goneDays = map[aggKey]bool{}
	x.goneCat = map[catKey]bool{}
}

func (x *index) reset() {
	x.mu.Lock()
	x.resetLocked()
	x.mu.Unlock()
}

func (x *index) observe(k keyInfo, size int64) {
	x.mu.Lock()
	x.observeLocked(k, size)
	x.mu.Unlock()
}

func (x *index) observeLocked(k keyInfo, size int64) {
	ak := aggKey{k.date, k.ns, k.wl}
	d := x.days[ak]
	if d == nil {
		d = &model.WorkloadDay{Date: k.date, Namespace: k.ns, Workload: k.wl, Type: k.wtype, Levels: map[string]int64{}, First: k.ts, Last: k.ts}
		x.days[ak] = d
	}
	d.Levels[k.level]++
	d.Bytes += size
	d.First = min(d.First, k.ts)
	d.Last = max(d.Last, k.ts)
	if k.wtype != "" {
		d.Type = k.wtype
	}
	if len(d.Pods) < maxPodsPerDay && !slices.Contains(d.Pods, k.pod) {
		d.Pods = append(d.Pods, k.pod)
	}
	x.dirtyDays[ak] = true
	delete(x.goneDays, ak)

	ck := catKey{k.ns, k.pod, k.container}
	c := x.cat[ck]
	if c == nil {
		c = &model.CatalogItem{Namespace: k.ns, Pod: k.pod, Container: k.container, FirstSeen: k.ts, LastSeen: k.ts}
		x.cat[ck] = c
	}
	c.Workload, c.WorkloadType = k.wl, k.wtype
	c.FirstSeen = min(c.FirstSeen, k.ts)
	c.LastSeen = max(c.LastSeen, k.ts)
	x.dirtyCat[ck] = true
	delete(x.goneCat, ck)
}

// flush persists what changed since the last flush.
func (x *index) flush(db *badgerdb.DB) error {
	type op struct {
		key, val []byte
	}
	x.mu.Lock()
	var ops []op
	for ak := range x.dirtyDays {
		b, _ := json.Marshal(x.days[ak])
		ops = append(ops, op{aggKeyBytes(ak), b})
	}
	for ak := range x.goneDays {
		ops = append(ops, op{aggKeyBytes(ak), nil})
	}
	for ck := range x.dirtyCat {
		b, _ := json.Marshal(x.cat[ck])
		ops = append(ops, op{catKeyBytes(ck), b})
	}
	for ck := range x.goneCat {
		ops = append(ops, op{catKeyBytes(ck), nil})
	}
	clear(x.dirtyDays)
	clear(x.goneDays)
	clear(x.dirtyCat)
	clear(x.goneCat)
	x.mu.Unlock()

	if len(ops) == 0 {
		return nil
	}
	wb := db.NewWriteBatch()
	defer wb.Cancel()
	for _, o := range ops {
		var err error
		if o.val == nil {
			err = wb.Delete(o.key)
		} else {
			err = wb.Set(o.key, o.val)
		}
		if err != nil {
			return err
		}
	}
	return wb.Flush()
}

func (x *index) load(db *badgerdb.DB) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.resetLocked()
	return db.View(func(txn *badgerdb.Txn) error {
		for _, prefix := range []string{aggPrefix, catPrefix} {
			opts := badgerdb.DefaultIteratorOptions
			opts.Prefix = []byte(prefix)
			it := txn.NewIterator(opts)
			for it.Rewind(); it.Valid(); it.Next() {
				err := it.Item().Value(func(v []byte) error {
					if prefix == aggPrefix {
						var d model.WorkloadDay
						if json.Unmarshal(v, &d) == nil {
							if d.Levels == nil {
								d.Levels = map[string]int64{}
							}
							x.days[aggKey{d.Date, d.Namespace, d.Workload}] = &d
						}
					} else {
						var c model.CatalogItem
						if json.Unmarshal(v, &c) == nil {
							x.cat[catKey{c.Namespace, c.Pod, c.Container}] = &c
						}
					}
					return nil
				})
				if err != nil {
					it.Close()
					return err
				}
			}
			it.Close()
		}
		return nil
	})
}

// removeDays drops rollups matching fn and forgets containers that no longer
// have logs on any remaining day.
func (x *index) removeDays(fn func(aggKey) bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for ak := range x.days {
		if fn(ak) {
			delete(x.days, ak)
			delete(x.dirtyDays, ak)
			x.goneDays[ak] = true
		}
	}
	earliest := map[[2]string]string{}
	for _, d := range x.days {
		for _, p := range d.Pods {
			k := [2]string{d.Namespace, p}
			if e, ok := earliest[k]; !ok || d.Date < e {
				earliest[k] = d.Date
			}
		}
	}
	for ck, c := range x.cat {
		e, ok := earliest[[2]string{ck.ns, ck.pod}]
		if !ok {
			delete(x.cat, ck)
			delete(x.dirtyCat, ck)
			x.goneCat[ck] = true
			continue
		}
		if start, err := time.ParseInLocation("2006-01-02", e, x.loc); err == nil && c.FirstSeen < start.UnixNano() {
			c.FirstSeen = start.UnixNano()
			x.dirtyCat[ck] = true
		}
	}
}

func (x *index) catalog() []model.CatalogItem {
	x.mu.Lock()
	out := make([]model.CatalogItem, 0, len(x.cat))
	for _, c := range x.cat {
		out = append(out, *c)
	}
	x.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Workload != b.Workload {
			return a.Workload < b.Workload
		}
		if a.Pod != b.Pod {
			return a.Pod < b.Pod
		}
		return a.Container < b.Container
	})
	return out
}

func (x *index) distinct(field func(*model.CatalogItem) string) []string {
	seen := map[string]bool{}
	x.mu.Lock()
	for _, c := range x.cat {
		if v := field(c); v != "" {
			seen[v] = true
		}
	}
	x.mu.Unlock()
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// dateStats returns lines and bytes per day, newest first.
func (x *index) dateStats() []model.DateBreakdown {
	m := map[string]*model.DateBreakdown{}
	x.mu.Lock()
	for _, d := range x.days {
		b := m[d.Date]
		if b == nil {
			b = &model.DateBreakdown{Date: d.Date}
			m[d.Date] = b
		}
		b.EntryCount += d.Lines()
		b.SizeBytes += d.Bytes
	}
	x.mu.Unlock()
	out := make([]model.DateBreakdown, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

// recap returns copies of the rollups between two dates (inclusive).
func (x *index) recap(from, to, ns string) []model.WorkloadDay {
	x.mu.Lock()
	defer x.mu.Unlock()
	out := []model.WorkloadDay{}
	for ak, d := range x.days {
		if (from != "" && ak.date < from) || (to != "" && ak.date > to) || (ns != "" && ak.ns != ns) {
			continue
		}
		c := *d
		c.Levels = make(map[string]int64, len(d.Levels))
		for l, n := range d.Levels {
			c.Levels[l] = n
		}
		c.Pods = slices.Clone(d.Pods)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Namespace+"/"+out[i].Workload < out[j].Namespace+"/"+out[j].Workload
	})
	return out
}

func aggKeyBytes(k aggKey) []byte {
	return []byte(aggPrefix + k.date + keySep + k.ns + keySep + k.wl)
}

func catKeyBytes(k catKey) []byte {
	return []byte(catPrefix + k.ns + keySep + k.pod + keySep + k.container)
}
