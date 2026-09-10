package badger

import (
	"os"
	"testing"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/model"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(Options{
		Path:        dir,
		Retention:   24 * time.Hour,
		MaxDisk:     1 << 30, // 1GB
		GCInterval:  1 * time.Hour, // long, we don't want GC during tests
		Compression: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWriteAndQuery(t *testing.T) {
	s := testStore(t)

	entries := []model.LogEntry{
		{
			Timestamp:    time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Date:         "2025-01-15",
			Namespace:    "production",
			Workload:     "api-server",
			WorkloadType: "deployment",
			Pod:          "api-server-7f8b-x2k1p",
			Container:    "app",
			Node:         "node-1",
			Stream:       "stdout",
			Level:        "INFO",
			Message:      "Request handled successfully",
		},
		{
			Timestamp:    time.Date(2025, 1, 15, 10, 30, 1, 0, time.UTC),
			Date:         "2025-01-15",
			Namespace:    "production",
			Workload:     "api-server",
			WorkloadType: "deployment",
			Pod:          "api-server-7f8b-y3m4n",
			Container:    "app",
			Node:         "node-2",
			Stream:       "stderr",
			Level:        "ERROR",
			Message:      "Connection timeout",
		},
		{
			Timestamp:    time.Date(2025, 1, 15, 10, 30, 2, 0, time.UTC),
			Date:         "2025-01-15",
			Namespace:    "database",
			Workload:     "postgres",
			WorkloadType: "statefulset",
			Pod:          "postgres-0",
			Container:    "postgres",
			Node:         "node-1",
			Stream:       "stdout",
			Level:        "INFO",
			Message:      "Checkpoint complete",
		},
	}

	if err := s.Write(entries); err != nil {
		t.Fatal("write:", err)
	}

	// Query all
	result, err := s.Query(model.QueryRequest{Date: "2025-01-15", Limit: 100})
	if err != nil {
		t.Fatal("query:", err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}

	// Query by namespace
	result, err = s.Query(model.QueryRequest{Date: "2025-01-15", Namespace: "production", Limit: 100})
	if err != nil {
		t.Fatal("query:", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 production entries, got %d", len(result.Entries))
	}

	// Query by workload
	result, err = s.Query(model.QueryRequest{Date: "2025-01-15", Namespace: "production", Workload: "api-server", Limit: 100})
	if err != nil {
		t.Fatal("query:", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 api-server entries, got %d", len(result.Entries))
	}

	// Query by level
	result, err = s.Query(model.QueryRequest{Date: "2025-01-15", Level: "ERROR", Limit: 100})
	if err != nil {
		t.Fatal("query:", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 ERROR entry, got %d", len(result.Entries))
	}

	// Query by search
	result, err = s.Query(model.QueryRequest{Date: "2025-01-15", Search: "timeout", Limit: 100})
	if err != nil {
		t.Fatal("query:", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry matching 'timeout', got %d", len(result.Entries))
	}
}

func TestDates(t *testing.T) {
	s := testStore(t)

	entries := []model.LogEntry{
		{Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), Date: "2025-01-15", Namespace: "ns", Workload: "w", Pod: "p", Container: "c", Message: "m1"},
		{Timestamp: time.Date(2025, 1, 16, 10, 0, 0, 0, time.UTC), Date: "2025-01-16", Namespace: "ns", Workload: "w", Pod: "p", Container: "c", Message: "m2"},
	}

	if err := s.Write(entries); err != nil {
		t.Fatal(err)
	}

	dates, err := s.Dates()
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 2 {
		t.Fatalf("expected 2 dates, got %d", len(dates))
	}
}

func TestNamespacesAndWorkloads(t *testing.T) {
	s := testStore(t)

	entries := []model.LogEntry{
		{Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), Date: "2025-01-15", Namespace: "prod", Workload: "api", Pod: "api-1", Container: "c", Message: "m"},
		{Timestamp: time.Date(2025, 1, 15, 10, 0, 1, 0, time.UTC), Date: "2025-01-15", Namespace: "prod", Workload: "worker", Pod: "worker-1", Container: "c", Message: "m"},
		{Timestamp: time.Date(2025, 1, 15, 10, 0, 2, 0, time.UTC), Date: "2025-01-15", Namespace: "staging", Workload: "api", Pod: "api-2", Container: "c", Message: "m"},
	}
	s.Write(entries)

	ns, _ := s.Namespaces()
	if len(ns) != 2 {
		t.Fatalf("expected 2 namespaces, got %d: %v", len(ns), ns)
	}

	wl, _ := s.Workloads("prod")
	if len(wl) != 2 {
		t.Fatalf("expected 2 workloads in prod, got %d: %v", len(wl), wl)
	}

	pods, _ := s.Pods("prod", "api")
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod for prod/api, got %d: %v", len(pods), pods)
	}
}

func TestDeleteAndReset(t *testing.T) {
	s := testStore(t)

	entries := []model.LogEntry{
		{Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), Date: "2025-01-15", Namespace: "ns", Workload: "w", Pod: "p", Container: "c", Message: "m1"},
		{Timestamp: time.Date(2025, 1, 16, 10, 0, 0, 0, time.UTC), Date: "2025-01-16", Namespace: "ns", Workload: "w", Pod: "p", Container: "c", Message: "m2"},
		{Timestamp: time.Date(2025, 1, 17, 10, 0, 0, 0, time.UTC), Date: "2025-01-17", Namespace: "ns", Workload: "w", Pod: "p", Container: "c", Message: "m3"},
	}
	s.Write(entries)

	// Delete before 2025-01-17
	deleted, err := s.Delete(model.DeleteRequest{BeforeDate: "2025-01-17"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted, got %d", deleted)
	}

	result, _ := s.Query(model.QueryRequest{Limit: 100})
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(result.Entries))
	}

	// Reset all
	err = s.ResetAll()
	if err != nil {
		t.Fatal(err)
	}

	result, _ = s.Query(model.QueryRequest{Limit: 100})
	if len(result.Entries) != 0 {
		t.Fatalf("expected 0 after reset, got %d", len(result.Entries))
	}
}

func TestOffsets(t *testing.T) {
	s := testStore(t)

	file := "/var/log/containers/test.log"

	// Initially 0
	offset, err := s.LoadOffset(file)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 {
		t.Fatalf("expected 0 offset, got %d", offset)
	}

	// Save and reload
	s.SaveOffset(file, 12345)
	offset, _ = s.LoadOffset(file)
	if offset != 12345 {
		t.Fatalf("expected 12345 offset, got %d", offset)
	}
}

func TestStorageInfo(t *testing.T) {
	s := testStore(t)

	entries := []model.LogEntry{
		{Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), Date: "2025-01-15", Namespace: "ns", Workload: "w", Pod: "p", Container: "c", Message: "hello"},
		{Timestamp: time.Date(2025, 1, 15, 10, 0, 1, 0, time.UTC), Date: "2025-01-15", Namespace: "ns", Workload: "w", Pod: "p", Container: "c", Message: "world"},
	}
	s.Write(entries)

	info, err := s.StorageInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.EntryCount != 2 {
		t.Fatalf("expected 2 entries, got %d", info.EntryCount)
	}
	if info.OldestDate != "2025-01-15" {
		t.Fatalf("expected oldest 2025-01-15, got %s", info.OldestDate)
	}
	if info.UsedBytes <= 0 {
		t.Fatal("expected positive used bytes")
	}
}

// Silence unused import warning for os
var _ = os.TempDir
