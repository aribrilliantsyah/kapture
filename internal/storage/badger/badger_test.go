package badger

import (
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	"github.com/ordinary/k8s-log-catcher/internal/model"
)

func openStore(t *testing.T, dir string, retention time.Duration) *Store {
	t.Helper()
	s, err := New(Options{
		Path:        dir,
		Node:        "node-1",
		Retention:   retention,
		MaxDisk:     1 << 30,
		GCInterval:  time.Hour, // no GC during tests
		Compression: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s := openStore(t, t.TempDir(), 0)
	t.Cleanup(func() { s.Close() })
	return s
}

func at(h, m, sec int) time.Time { return time.Date(2025, 1, 15, h, m, sec, 0, time.UTC) }

func entry(ts time.Time, ns, wl, pod, level, msg string) model.LogEntry {
	return model.LogEntry{Timestamp: ts, Namespace: ns, Workload: wl, WorkloadType: "deployment",
		Pod: pod, Container: "app", Stream: "stdout", Level: level, Message: msg}
}

func messages(r *model.QueryResult) []string {
	out := make([]string, len(r.Entries))
	for i, e := range r.Entries {
		out[i] = e.Message
	}
	return out
}

func mustQuery(t *testing.T, s *Store, req model.QueryRequest) *model.QueryResult {
	t.Helper()
	r, err := s.Query(req)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWriteAndQuery(t *testing.T) {
	s := testStore(t)
	if err := s.Write([]model.LogEntry{
		entry(at(10, 30, 0), "production", "api-server", "api-server-x2k1p", "INFO", "Request handled successfully"),
		entry(at(10, 30, 1), "production", "api-server", "api-server-y3m4n", "ERROR", "Connection timeout"),
		entry(at(10, 30, 2), "database", "postgres", "postgres-0", "INFO", "Checkpoint complete"),
	}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		req  model.QueryRequest
		want int
	}{
		{"all", model.QueryRequest{Date: "2025-01-15"}, 3},
		{"namespace", model.QueryRequest{Date: "2025-01-15", Namespace: "production"}, 2},
		{"workload", model.QueryRequest{Namespace: "production", Workload: "api-server"}, 2},
		{"pod", model.QueryRequest{Pod: "postgres-0"}, 1},
		{"level", model.QueryRequest{Level: "error"}, 1},
		{"levels", model.QueryRequest{Level: "ERROR,INFO"}, 3},
		{"search", model.QueryRequest{Search: "TIMEOUT"}, 1},
		{"search field", model.QueryRequest{Search: "ns:database checkpoint"}, 1},
		{"exclude", model.QueryRequest{ExcludeNS: []string{"production"}}, 1},
		{"regex", model.QueryRequest{Regex: `^Conn.*out$`}, 1},
		{"other date", model.QueryRequest{Date: "2025-01-16"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := len(mustQuery(t, s, c.req).Entries); got != c.want {
				t.Fatalf("got %d entries, want %d", got, c.want)
			}
		})
	}

	e := mustQuery(t, s, model.QueryRequest{Level: "ERROR"}).Entries[0]
	if e.Node != "node-1" || e.Stream != "stdout" || e.Date != "2025-01-15" || e.WorkloadType != "deployment" {
		t.Fatalf("entry fields not restored: %+v", e)
	}
}

// Logs must come back in time order across namespaces, not grouped by key prefix.
func TestQueryIsChronological(t *testing.T) {
	s := testStore(t)
	s.Write([]model.LogEntry{
		entry(at(10, 0, 1), "zeta", "a", "a-0", "INFO", "1"),
		entry(at(10, 0, 2), "alpha", "b", "b-0", "INFO", "2"),
		entry(at(10, 0, 3), "zeta", "a", "a-0", "INFO", "3"),
		entry(at(10, 0, 4), "alpha", "b", "b-0", "INFO", "4"),
		entry(time.Date(2025, 1, 16, 0, 0, 1, 0, time.UTC), "alpha", "b", "b-0", "INFO", "5"),
	})

	desc := mustQuery(t, s, model.QueryRequest{Limit: 2})
	if got := messages(desc); len(got) != 2 || got[0] != "5" || got[1] != "4" {
		t.Fatalf("desc page 1 = %v, want [5 4]", got)
	}
	next := mustQuery(t, s, model.QueryRequest{Limit: 2, Cursor: desc.NextCursor})
	if got := messages(next); len(got) != 2 || got[0] != "3" || got[1] != "2" {
		t.Fatalf("desc page 2 = %v, want [3 2]", got)
	}

	asc := mustQuery(t, s, model.QueryRequest{Sort: "asc", Limit: 3})
	if got := messages(asc); len(got) != 3 || got[0] != "1" || got[2] != "3" {
		t.Fatalf("asc = %v, want [1 2 3]", got)
	}

	from, to := at(10, 0, 2), at(10, 0, 3)
	window := mustQuery(t, s, model.QueryRequest{From: &from, To: &to})
	if got := messages(window); len(got) != 2 || got[0] != "3" || got[1] != "2" {
		t.Fatalf("window = %v, want [3 2]", got)
	}
}

func TestCatalog(t *testing.T) {
	s := testStore(t)
	s.Write([]model.LogEntry{
		entry(at(10, 0, 0), "prod", "api", "api-1", "INFO", "m"),
		entry(at(10, 0, 1), "prod", "worker", "worker-1", "INFO", "m"),
		entry(at(10, 0, 2), "staging", "api", "api-2", "INFO", "m"),
		entry(at(10, 0, 3), "prod", "api", "api-1", "INFO", "m"),
	})

	if ns, _ := s.Namespaces(); len(ns) != 2 {
		t.Fatalf("namespaces = %v", ns)
	}
	if wl, _ := s.Workloads("prod"); len(wl) != 2 {
		t.Fatalf("workloads = %v", wl)
	}
	if pods, _ := s.Pods("prod", "api"); len(pods) != 1 {
		t.Fatalf("pods = %v", pods)
	}
	cat := s.Catalog()
	if len(cat) != 3 || cat[0].Pod != "api-1" || cat[0].Node != "node-1" ||
		cat[0].FirstSeen != at(10, 0, 0).UnixNano() || cat[0].LastSeen != at(10, 0, 3).UnixNano() {
		t.Fatalf("catalog = %+v", cat)
	}
}

// Pods replaced by a rollout stay in the catalog under the same workload, and
// the index survives a restart without rescanning logs.
func TestIndexKeepsHistoryAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir, 0)
	day1 := time.Date(2025, 1, 14, 9, 0, 0, 0, time.UTC)
	s.Write([]model.LogEntry{
		entry(day1, "prod", "api", "api-7f8b9c6d4-aaaaa", "INFO", "old pod"),
		entry(day1.Add(time.Minute), "prod", "api", "api-7f8b9c6d4-aaaaa", "ERROR", "old pod failed"),
		entry(at(10, 0, 0), "prod", "api", "api-5c6d7f8b9-bbbbb", "INFO", "new pod"),
	})
	s.Close()

	s = openStore(t, dir, 0)
	defer s.Close()
	if pods, _ := s.Pods("prod", "api"); len(pods) != 2 {
		t.Fatalf("pods after restart = %v", pods)
	}
	recap := s.Recap("2025-01-14", "2025-01-15", "")
	if len(recap) != 2 || recap[0].Date != "2025-01-14" || recap[0].Levels["ERROR"] != 1 || recap[0].Lines() != 2 || recap[1].Pods[0] != "api-5c6d7f8b9-bbbbb" {
		t.Fatalf("recap = %+v", recap)
	}

	// Dropping the older day forgets the pod that only lived that day.
	if _, err := s.Delete(model.DeleteRequest{Date: "2025-01-14"}); err != nil {
		t.Fatal(err)
	}
	if pods, _ := s.Pods("prod", "api"); len(pods) != 1 || pods[0] != "api-5c6d7f8b9-bbbbb" {
		t.Fatalf("pods after delete = %v", pods)
	}
	if info, _ := s.StorageInfo(); info.EntryCount != 1 || info.Retention != "unlimited" {
		t.Fatalf("storage info = %+v", info)
	}
}

// Stores written by the previous version (schema 2) keep their logs; the
// index is built from the keys once.
func TestMigrationFromSchema2BuildsIndex(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir, 0)
	s.Write([]model.LogEntry{entry(at(10, 0, 0), "prod", "api", "api-1", "WARN", "kept")})
	s.flushIndex()
	s.db.Update(func(txn *badgerdb.Txn) error {
		for _, p := range []string{aggPrefix, catPrefix} {
			it := txn.NewIterator(badgerdb.IteratorOptions{Prefix: []byte(p)})
			var keys [][]byte
			for it.Rewind(); it.Valid(); it.Next() {
				keys = append(keys, it.Item().KeyCopy(nil))
			}
			it.Close()
			for _, k := range keys {
				txn.Delete(k)
			}
		}
		return txn.Set([]byte(schemaKey), []byte("2"))
	})
	s.Close()

	s = openStore(t, dir, 0)
	defer s.Close()
	if got := messages(mustQuery(t, s, model.QueryRequest{})); len(got) != 1 || got[0] != "kept" {
		t.Fatalf("logs lost in migration: %v", got)
	}
	if r := s.Recap("", "", ""); len(r) != 1 || r[0].Levels["WARN"] != 1 {
		t.Fatalf("index not rebuilt: %+v", r)
	}
}

func TestDates(t *testing.T) {
	s := testStore(t)
	s.Write([]model.LogEntry{
		entry(time.Date(2025, 1, 16, 10, 0, 0, 0, time.UTC), "ns", "w", "p", "INFO", "m2"),
		entry(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), "ns", "w", "p", "INFO", "m1"),
	})
	dates, err := s.Dates()
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 2 || dates[0] != "2025-01-15" || dates[1] != "2025-01-16" {
		t.Fatalf("dates = %v", dates)
	}
}

func TestDeleteAndReset(t *testing.T) {
	s := testStore(t)
	day := func(d int) time.Time { return time.Date(2025, 1, d, 10, 0, 0, 0, time.UTC) }
	s.Write([]model.LogEntry{
		entry(day(15), "ns", "w", "p", "INFO", "m1"),
		entry(day(16), "ns", "w", "p", "INFO", "m2"),
		entry(day(17), "ns", "w", "p", "INFO", "m3"),
		entry(day(18), "ns", "w", "p", "INFO", "m4"),
		entry(day(18), "other", "w", "p", "INFO", "m5"),
	})
	s.SaveOffset("/var/log/containers/x.log", 42, 7)

	if n, err := s.Delete(model.DeleteRequest{BeforeDate: "2025-01-16"}); err != nil || n != 1 {
		t.Fatalf("delete before: n=%d err=%v", n, err)
	}
	if n, err := s.Delete(model.DeleteRequest{Date: "2025-01-17"}); err != nil || n != 1 {
		t.Fatalf("delete date: n=%d err=%v", n, err)
	}
	if n, err := s.Delete(model.DeleteRequest{Namespace: "other"}); err != nil || n != 1 {
		t.Fatalf("delete namespace: n=%d err=%v", n, err)
	}
	if got := messages(mustQuery(t, s, model.QueryRequest{})); len(got) != 2 || got[0] != "m4" || got[1] != "m2" {
		t.Fatalf("remaining = %v, want [m4 m2]", got)
	}
	if _, err := s.Delete(model.DeleteRequest{}); err == nil {
		t.Fatal("empty delete request should fail")
	}

	if _, err := s.Delete(model.DeleteRequest{All: true}); err != nil {
		t.Fatal(err)
	}
	if n := len(mustQuery(t, s, model.QueryRequest{}).Entries); n != 0 {
		t.Fatalf("expected 0 after reset, got %d", n)
	}
	if len(s.Catalog()) != 0 {
		t.Fatal("catalog not cleared by reset")
	}
	// Offsets survive a reset, otherwise every file would be ingested again.
	if off, ino, _ := s.LoadOffset("/var/log/containers/x.log"); off != 42 || ino != 7 {
		t.Fatalf("offset after reset = %d/%d, want 42/7", off, ino)
	}
}

func TestOffsets(t *testing.T) {
	s := testStore(t)
	file := "/var/log/containers/test.log"

	if off, ino, err := s.LoadOffset(file); err != nil || off != 0 || ino != 0 {
		t.Fatalf("initial offset = %d/%d err=%v", off, ino, err)
	}
	s.SaveOffset(file, 12345, 99)
	if off, ino, _ := s.LoadOffset(file); off != 12345 || ino != 99 {
		t.Fatalf("offset = %d/%d, want 12345/99", off, ino)
	}
	s.DeleteOffset(file)
	if off, _, _ := s.LoadOffset(file); off != 0 {
		t.Fatalf("offset after delete = %d", off)
	}
}

func TestStorageInfo(t *testing.T) {
	s := testStore(t)
	s.Write([]model.LogEntry{
		entry(at(10, 0, 0), "ns", "w", "p", "INFO", "hello"),
		entry(at(10, 0, 1), "ns", "w", "p", "INFO", "world"),
	})
	info, err := s.StorageInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.EntryCount != 2 || info.OldestDate != "2025-01-15" || len(info.Dates) != 1 || info.Dates[0].EntryCount != 2 {
		t.Fatalf("info = %+v", info)
	}
}

func TestVolume(t *testing.T) {
	s := testStore(t)
	s.Write([]model.LogEntry{
		entry(at(10, 0, 0), "ns", "api", "p", "INFO", "a"),
		entry(at(10, 0, 30), "ns", "api", "p", "ERROR", "b"),
		entry(at(10, 1, 10), "ns", "api", "p", "ERROR", "c"),
		entry(at(10, 1, 20), "ns", "db", "q", "WARN", "d"),
	})
	from, to := at(10, 0, 0), at(10, 2, 0)
	v, err := s.Volume(model.QueryRequest{From: &from, To: &to, Buckets: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Buckets) != 2 || v.Buckets[0].Counts["ERROR"] != 1 || v.Buckets[1].Counts["ERROR"] != 1 || v.Buckets[1].Counts["WARN"] != 1 {
		t.Fatalf("buckets = %+v", v.Buckets)
	}
	if v.Totals["INFO"] != 1 || len(v.TopErrors) != 1 || v.TopErrors[0].Workload != "api" || v.TopErrors[0].Count != 2 {
		t.Fatalf("totals = %v top = %v", v.Totals, v.TopErrors)
	}
}

func TestExpiredEntriesAreSkipped(t *testing.T) {
	s := openStore(t, t.TempDir(), time.Hour)
	defer s.Close()
	s.Write([]model.LogEntry{
		entry(time.Now().Add(-2*time.Hour), "ns", "w", "p", "INFO", "old"),
		entry(time.Now(), "ns", "w", "p", "INFO", "new"),
	})
	if got := messages(mustQuery(t, s, model.QueryRequest{})); len(got) != 1 || got[0] != "new" {
		t.Fatalf("got %v, want [new]", got)
	}
}

// Data written with the old namespace-first layout is cleared on upgrade.
func TestMigrationClearsLegacyLayout(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir, 0)
	s.db.Update(func(txn *badgerdb.Txn) error {
		txn.Delete([]byte(schemaKey))
		return txn.Set([]byte("2025-01-15:prod:api:api-1:app:0000000000000001:00000001"), []byte(`{}`))
	})
	s.SaveOffset("/var/log/containers/x.log", 10, 0)
	s.Close()

	s = openStore(t, dir, 0)
	defer s.Close()
	if dates, _ := s.Dates(); len(dates) != 0 {
		t.Fatalf("legacy data not cleared: %v", dates)
	}
	if off, _, _ := s.LoadOffset("/var/log/containers/x.log"); off != 0 {
		t.Fatal("offsets must be cleared so logs are re-ingested")
	}
	s.Write([]model.LogEntry{entry(at(10, 0, 0), "ns", "w", "p", "INFO", "m")})
	if n := len(mustQuery(t, s, model.QueryRequest{}).Entries); n != 1 {
		t.Fatalf("expected 1 entry after migration, got %d", n)
	}
}

// Re-reading a file (same seq) overwrites instead of duplicating.
func TestSameSeqIsIdempotent(t *testing.T) {
	s := testStore(t)
	e := entry(at(10, 0, 0), "ns", "w", "p", "INFO", "m")
	e.Seq = 100
	s.Write([]model.LogEntry{e})
	s.Write([]model.LogEntry{e})
	if n := len(mustQuery(t, s, model.QueryRequest{}).Entries); n != 1 {
		t.Fatalf("expected 1 entry, got %d", n)
	}
}

func openStoreIn(t *testing.T, dir string, loc *time.Location) *Store {
	t.Helper()
	s, err := New(Options{Path: dir, Node: "node-1", Location: loc, GCInterval: time.Hour, Compression: "none"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Days follow the configured zone; changing the zone moves stored logs to
// their local day once, keeping every line.
func TestLocalTimezoneDays(t *testing.T) {
	jkt, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skip("no tzdata")
	}
	dir := t.TempDir()
	late := time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC) // 03:00 WIB on the 16th

	s := openStoreIn(t, dir, nil)
	s.Write([]model.LogEntry{entry(late, "prod", "api", "api-1", "ERROR", "late night")})
	if d, _ := s.Dates(); len(d) != 1 || d[0] != "2025-01-15" {
		t.Fatalf("UTC dates = %v", d)
	}
	s.Close()

	s = openStoreIn(t, dir, jkt)
	defer s.Close()
	if d, _ := s.Dates(); len(d) != 1 || d[0] != "2025-01-16" {
		t.Fatalf("WIB dates after zone change = %v", d)
	}
	if got := messages(mustQuery(t, s, model.QueryRequest{Date: "2025-01-16"})); len(got) != 1 || got[0] != "late night" {
		t.Fatalf("query by local date = %v", got)
	}
	if r := s.Recap("2025-01-16", "2025-01-16", ""); len(r) != 1 || r[0].Levels["ERROR"] != 1 {
		t.Fatalf("recap = %+v", r)
	}
	if info, _ := s.StorageInfo(); info.Timezone != "Asia/Jakarta" || info.EntryCount != 1 {
		t.Fatalf("info = %+v", info)
	}

	// New lines land on their WIB day: 18:00 UTC on the 16th is 01:00 WIB on the 17th.
	s.Write([]model.LogEntry{entry(time.Date(2025, 1, 16, 18, 0, 0, 0, time.UTC), "prod", "api", "api-1", "INFO", "next day")})
	if d, _ := s.Dates(); len(d) != 2 || d[1] != "2025-01-17" {
		t.Fatalf("dates = %v", d)
	}
	from := time.Date(2025, 1, 16, 0, 0, 0, 0, jkt)
	to := from.Add(48 * time.Hour)
	if got := messages(mustQuery(t, s, model.QueryRequest{From: &from, To: &to})); len(got) != 2 {
		t.Fatalf("range across local days = %v", got)
	}
}
