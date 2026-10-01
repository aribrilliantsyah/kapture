package badger

import (
	"bytes"
	"testing"
	"time"

	"github.com/aribrilliantsyah/kapture/internal/model"
)

func TestBackupRestoreAcrossZones(t *testing.T) {
	src := testStore(t)
	if err := src.Write([]model.LogEntry{
		entry(time.Date(2025, 1, 14, 20, 0, 0, 0, time.UTC), "shop", "api", "api-1", "INFO", "late on the 14th UTC"),
		entry(at(10, 0, 0), "shop", "api", "api-1", "ERROR", "boom"),
		entry(at(11, 0, 0), "ops", "cron", "cron-1", "INFO", "tick"),
	}); err != nil {
		t.Fatal(err)
	}
	src.SaveOffset("/var/log/containers/x.log", 42, 7)

	var all, day bytes.Buffer
	if n, err := src.Backup(&all, "", ""); err != nil || n != 3 {
		t.Fatalf("backup: n=%d err=%v", n, err)
	}
	if n, err := src.Backup(&day, "2025-01-15", "2025-01-15"); err != nil || n != 2 {
		t.Fatalf("backup of one day: n=%d err=%v", n, err)
	}

	// Restore on a node in another zone: 20:00 UTC on the 14th is the 15th in Jakarta.
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	dst, err := New(Options{Path: t.TempDir(), Node: "laptop", Location: jkt, MaxDisk: 1 << 30, GCInterval: time.Hour, Compression: "none"})
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	for i := 0; i < 2; i++ { // restoring twice must not duplicate anything
		if n, err := dst.Restore(bytes.NewReader(all.Bytes())); err != nil || n != 3 {
			t.Fatalf("restore %d: n=%d err=%v", i, n, err)
		}
	}
	if dates, _ := dst.Dates(); len(dates) != 1 || dates[0] != "2025-01-15" {
		t.Fatalf("expected every line on 2025-01-15 in Jakarta, got %v", dates)
	}
	info, _ := dst.StorageInfo()
	if info.EntryCount != 3 {
		t.Fatalf("expected 3 lines in the index, got %d", info.EntryCount)
	}
	if len(dst.Catalog()) != 2 {
		t.Fatalf("expected 2 containers in the catalog, got %d", len(dst.Catalog()))
	}
	r := mustQuery(t, dst, model.QueryRequest{Level: "ERROR"})
	if len(r.Entries) != 1 || r.Entries[0].Message != "boom" || r.Entries[0].Node != "laptop" {
		t.Fatalf("query after restore: %+v", r.Entries)
	}
	// Read positions belong to the source node and are never restored.
	if off, _, _ := dst.LoadOffset("/var/log/containers/x.log"); off != 0 {
		t.Fatalf("offset was restored: %d", off)
	}

	if _, err := dst.Restore(bytes.NewReader([]byte("not a backup at all"))); err == nil {
		t.Fatal("garbage accepted as a backup")
	}
}
