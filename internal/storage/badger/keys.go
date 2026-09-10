package badger

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// Key format: <date>:<namespace>:<workload>:<pod>:<container>:<timestamp_nano>:<seq>
// This gives us efficient prefix scans by date, namespace, workload, etc.

const (
	keySep       = ":"
	offsetPrefix = "_offset:"
	metaPrefix   = "_meta:"
)

// buildKey constructs a storage key from log entry fields.
func buildKey(date, namespace, workload, pod, container string, tsNano int64, seq uint32) []byte {
	// Use fixed-width hex for timestamp to ensure lexicographic = chronological order
	key := fmt.Sprintf("%s:%s:%s:%s:%s:%016x:%08x",
		date, namespace, workload, pod, container, uint64(tsNano), seq)
	return []byte(key)
}

// parseKey extracts fields from a storage key.
func parseKey(key []byte) (date, namespace, workload, pod, container string, tsNano int64, err error) {
	parts := strings.SplitN(string(key), keySep, 7)
	if len(parts) < 7 {
		return "", "", "", "", "", 0, fmt.Errorf("invalid key: %s", key)
	}
	date = parts[0]
	namespace = parts[1]
	workload = parts[2]
	pod = parts[3]
	container = parts[4]

	var ts uint64
	_, err = fmt.Sscanf(parts[5], "%x", &ts)
	if err != nil {
		return "", "", "", "", "", 0, fmt.Errorf("invalid timestamp in key: %s", key)
	}
	tsNano = int64(ts)
	return date, namespace, workload, pod, container, tsNano, nil
}

// buildDatePrefix returns a key prefix for scanning a specific date.
func buildDatePrefix(date string) []byte {
	return []byte(date + keySep)
}

// buildNSPrefix returns a key prefix for scanning a specific date + namespace.
func buildNSPrefix(date, namespace string) []byte {
	return []byte(date + keySep + namespace + keySep)
}

// buildWorkloadPrefix returns a key prefix for date + namespace + workload.
func buildWorkloadPrefix(date, namespace, workload string) []byte {
	return []byte(date + keySep + namespace + keySep + workload + keySep)
}

// buildPodPrefix returns a key prefix for date + namespace + workload + pod.
func buildPodPrefix(date, namespace, workload, pod string) []byte {
	return []byte(date + keySep + namespace + keySep + workload + keySep + pod + keySep)
}

// offsetKey returns the key for storing a file offset.
func offsetKey(file string) []byte {
	return []byte(offsetPrefix + file)
}

// encodeOffset converts an int64 offset to bytes.
func encodeOffset(offset int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(offset))
	return b
}

// decodeOffset converts bytes to an int64 offset.
func decodeOffset(b []byte) int64 {
	if len(b) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b))
}

// dateFromTime extracts a YYYY-MM-DD string from a time.
func dateFromTime(t time.Time) string {
	return t.Format("2006-01-02")
}
