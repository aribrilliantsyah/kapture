package storage

import (
	"io"

	"github.com/aribrilliantsyah/kapture/internal/model"
)

// Store defines the interface for log storage.
type Store interface {
	// Write stores a batch of log entries.
	Write(entries []model.LogEntry) error

	// Query retrieves log entries matching the request, in time order.
	Query(req model.QueryRequest) (*model.QueryResult, error)

	// Volume returns a per-level log-volume histogram for the request.
	Volume(req model.QueryRequest) (*model.VolumeResult, error)

	// Catalog lists every container that has stored logs, including replaced pods.
	Catalog() []model.CatalogItem

	// Recap returns daily per-workload rollups between two dates (inclusive).
	Recap(from, to, namespace string) []model.WorkloadDay

	// Dates returns all dates that have log data, oldest first.
	Dates() ([]string, error)

	// Namespaces returns all unique namespaces.
	Namespaces() ([]string, error)

	// Workloads returns all unique workloads, optionally filtered by namespace.
	Workloads(namespace string) ([]string, error)

	// Pods returns all unique pods, optionally filtered by namespace and workload.
	Pods(namespace, workload string) ([]string, error)

	// Delete removes log entries matching the request (req.All = reset).
	Delete(req model.DeleteRequest) (int64, error)

	// Backup writes the log lines of the days between from and to (inclusive,
	// both optional) and returns how many were written.
	Backup(w io.Writer, from, to string) (int64, error)

	// Restore loads a backup, from any node, and returns how many lines it added.
	Restore(r io.Reader) (int64, error)

	// StorageInfo returns current storage usage statistics.
	StorageInfo() (*model.StorageInfo, error)

	// SaveOffset persists the read position and inode of a log file.
	SaveOffset(file string, offset int64, inode uint64) error

	// LoadOffset retrieves the read position and inode of a log file.
	LoadOffset(file string) (int64, uint64, error)

	// DeleteOffset forgets a log file.
	DeleteOffset(file string) error

	// Close shuts down the storage.
	Close() error
}
