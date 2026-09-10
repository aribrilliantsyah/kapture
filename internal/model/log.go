package model

import (
	"encoding/json"
	"time"
)

// LogEntry represents a single log line captured from a Kubernetes container.
type LogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Date         string    `json:"date"`          // YYYY-MM-DD
	Namespace    string    `json:"namespace"`
	Workload     string    `json:"workload"`      // extracted workload name
	WorkloadType string    `json:"workload_type"` // deployment, statefulset, daemonset, job, cronjob, pod
	Pod          string    `json:"pod"`
	Container    string    `json:"container"`
	Node         string    `json:"node"`
	Stream       string    `json:"stream"` // stdout / stderr
	Level        string    `json:"level"`  // DEBUG, INFO, WARN, ERROR, FATAL
	Message      string    `json:"message"`
}

// MarshalJSON implements custom JSON marshalling with unix nano timestamp.
func (e LogEntry) MarshalJSON() ([]byte, error) {
	type Alias LogEntry
	return json.Marshal(&struct {
		TimestampNano int64  `json:"timestamp_nano"`
		TimestampStr  string `json:"timestamp"`
		Alias
	}{
		TimestampNano: e.Timestamp.UnixNano(),
		TimestampStr:  e.Timestamp.Format(time.RFC3339Nano),
		Alias:         Alias(e),
	})
}

// QueryRequest holds filter parameters for querying logs.
type QueryRequest struct {
	Date         string   `json:"date"`          // YYYY-MM-DD
	From         *time.Time `json:"from"`
	To           *time.Time `json:"to"`
	Namespace    string   `json:"namespace"`
	Workload     string   `json:"workload"`
	WorkloadType string   `json:"workload_type"`
	Pod          string   `json:"pod"`
	Container    string   `json:"container"`
	Level        string   `json:"level"`
	Search       string   `json:"search"`
	Regex        string   `json:"regex"`
	ExcludeNS    []string `json:"exclude_ns"`
	Limit        int      `json:"limit"`
	Cursor       string   `json:"cursor"`
	Sort         string   `json:"sort"` // asc / desc
}

// DeleteRequest holds parameters for deleting logs.
type DeleteRequest struct {
	BeforeDate string `json:"before_date"`
	Namespace  string `json:"namespace"`
	Workload   string `json:"workload"`
	All        bool   `json:"all"`
}

// DateBreakdown shows log stats for a specific date.
type DateBreakdown struct {
	Date       string `json:"date"`
	EntryCount int64  `json:"entry_count"`
	SizeBytes  int64  `json:"size_bytes"`
}

// StorageInfo reports storage usage.
type StorageInfo struct {
	UsedBytes  int64           `json:"used_bytes"`
	MaxBytes   int64           `json:"max_bytes"`
	EntryCount int64           `json:"entry_count"`
	OldestDate string          `json:"oldest_date"`
	NewestDate string          `json:"newest_date"`
	Dates      []DateBreakdown `json:"dates"`
}

// QueryResult wraps a page of log entries with pagination cursor.
type QueryResult struct {
	Entries    []LogEntry `json:"entries"`
	NextCursor string     `json:"next_cursor,omitempty"`
	Total      int64      `json:"total"`
}
