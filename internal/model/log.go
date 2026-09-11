package model

import (
	"encoding/json"
	"strings"
	"time"
)

// Levels lists the normalized log levels in ascending severity.
var Levels = []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

// LogEntry represents a single log line captured from a Kubernetes container.
type LogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Date         string    `json:"date"` // YYYY-MM-DD (UTC)
	Namespace    string    `json:"namespace"`
	Workload     string    `json:"workload"`      // extracted workload name
	WorkloadType string    `json:"workload_type"` // deployment, statefulset, daemonset, job, cronjob, pod
	Pod          string    `json:"pod"`
	Container    string    `json:"container"`
	Node         string    `json:"node"`
	Stream       string    `json:"stream"` // stdout / stderr
	Level        string    `json:"level"`  // DEBUG, INFO, WARN, ERROR, FATAL
	Message      string    `json:"message"`
	// Seq disambiguates entries sharing a timestamp. The agent sets it from the
	// line's byte offset so re-reading a file overwrites instead of duplicating.
	Seq uint64 `json:"seq,omitempty"`
}

// MarshalJSON adds a unix-nano timestamp next to the RFC3339 one.
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

// DeleteRequest holds parameters for deleting logs.
type DeleteRequest struct {
	Date       string `json:"date"`        // delete exactly this date
	BeforeDate string `json:"before_date"` // delete every date before this one
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

// StorageInfo reports storage usage of one agent, or the sum over all agents.
type StorageInfo struct {
	Node       string          `json:"node,omitempty"`
	UsedBytes  int64           `json:"used_bytes"`
	MaxBytes   int64           `json:"max_bytes"`
	EntryCount int64           `json:"entry_count"`
	OldestDate string          `json:"oldest_date"`
	NewestDate string          `json:"newest_date"`
	Retention  string          `json:"retention,omitempty"`
	Timezone   string          `json:"timezone,omitempty"`
	Dates      []DateBreakdown `json:"dates"`
	Nodes      []StorageInfo   `json:"nodes,omitempty"` // per-agent breakdown (aggregator only)
}

// QueryResult wraps a page of log entries with a pagination cursor.
type QueryResult struct {
	Entries []LogEntry `json:"entries"`
	// NextCursor is the unix-nano timestamp to pass as `cursor` for the next page.
	NextCursor string `json:"next_cursor,omitempty"`
	// Partial is set when the scan budget ran out before `limit` matches were found.
	Partial bool     `json:"partial,omitempty"`
	Errors  []string `json:"errors,omitempty"` // unreachable agents (aggregator only)
}

// CatalogItem describes one container that has logs stored. Pods of a
// workload keep their entry after they are replaced, so history stays browsable.
type CatalogItem struct {
	Node         string `json:"node"`
	Namespace    string `json:"namespace"`
	Workload     string `json:"workload"`
	WorkloadType string `json:"workload_type"`
	Pod          string `json:"pod"`
	Container    string `json:"container"`
	FirstSeen    int64  `json:"first_seen"` // unix nano
	LastSeen     int64  `json:"last_seen"`  // unix nano
}

// WorkloadDay is the rollup of one workload's logs on one day (UTC). Agents
// maintain it on write; the aggregator sums the agents' rollups.
type WorkloadDay struct {
	Date      string           `json:"date"`
	Namespace string           `json:"namespace"`
	Workload  string           `json:"workload"`
	Type      string           `json:"workload_type"`
	Levels    map[string]int64 `json:"levels"`
	Bytes     int64            `json:"bytes"`
	Pods      []string         `json:"pods"`
	First     int64            `json:"first"` // unix nano of the first line that day
	Last      int64            `json:"last"`
}

// Lines is the total number of lines in the rollup.
func (d *WorkloadDay) Lines() int64 {
	var n int64
	for _, c := range d.Levels {
		n += c
	}
	return n
}

// NormalizeLevel maps level spellings onto Levels; unknown values become INFO.
func NormalizeLevel(l string) string {
	switch strings.ToUpper(strings.TrimSpace(l)) {
	case "DEBUG", "TRACE", "DBG":
		return "DEBUG"
	case "WARN", "WARNING", "WRN":
		return "WARN"
	case "ERROR", "ERR":
		return "ERROR"
	case "FATAL", "PANIC", "CRITICAL":
		return "FATAL"
	default:
		return "INFO"
	}
}

// VolumeBucket counts log lines per level inside one time bucket.
type VolumeBucket struct {
	Start  int64            `json:"start"` // unix nano
	Counts map[string]int64 `json:"counts"`
}

// WorkloadCount is a per-workload line count used for "top" lists.
type WorkloadCount struct {
	Namespace string `json:"namespace"`
	Workload  string `json:"workload"`
	Count     int64  `json:"count"`
}

// VolumeResult is a log-volume histogram plus totals.
type VolumeResult struct {
	From        int64            `json:"from"`
	To          int64            `json:"to"`
	BucketNanos int64            `json:"bucket_nanos"`
	Buckets     []VolumeBucket   `json:"buckets"`
	Totals      map[string]int64 `json:"totals"`
	TopErrors   []WorkloadCount  `json:"top_errors"`
	Partial     bool             `json:"partial,omitempty"`
	Errors      []string         `json:"errors,omitempty"`
}
