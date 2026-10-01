package hub

import (
	"testing"

	"github.com/aribrilliantsyah/kapture/internal/logfilter"
	"github.com/aribrilliantsyah/kapture/internal/model"
)

func TestPublishFilters(t *testing.T) {
	h := New()
	f, _ := logfilter.New(model.QueryRequest{Namespace: "prod", Level: "ERROR", Search: "timeout"})
	s := h.Subscribe(f)
	defer h.Unsubscribe(s)

	h.Publish([]model.LogEntry{
		{Namespace: "prod", Level: "ERROR", Message: "db timeout"},
		{Namespace: "prod", Level: "INFO", Message: "timeout retry"},
		{Namespace: "dev", Level: "ERROR", Message: "timeout"},
		{Namespace: "prod", Level: "ERROR", Message: "disk full"},
	})
	if len(s.C) != 1 || (<-s.C).Message != "db timeout" {
		t.Fatal("expected exactly the matching entry")
	}
}

func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	h := New()
	f, _ := logfilter.New(model.QueryRequest{})
	s := h.Subscribe(f)
	batch := make([]model.LogEntry, subBuffer+10)
	h.Publish(batch) // must return even though the buffer overflows
	if s.Dropped.Load() != 10 {
		t.Fatalf("dropped = %d, want 10", s.Dropped.Load())
	}
}
