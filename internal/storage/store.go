package storage

import (
	"github.com/ordinary/k8s-log-catcher/internal/model"
)

// Store defines the interface for log storage.
type Store interface {
	// Write stores a batch of log entries.
	Write(entries []model.LogEntry) error

	// Query retrieves log entries matching the request.
	Query(req model.QueryRequest) (*model.QueryResult, error)

	// Dates returns all dates that have log data.
	Dates() ([]string, error)

	// Namespaces returns all unique namespaces.
	Namespaces() ([]string, error)

	// Workloads returns all unique workloads, optionally filtered by namespace.
	Workloads(namespace string) ([]string, error)

	// Pods returns all unique pods, optionally filtered by namespace and workload.
	Pods(namespace, workload string) ([]string, error)

	// Delete removes log entries matching the request.
	Delete(req model.DeleteRequest) (int64, error)

	// ResetAll drops all log data.
	ResetAll() error

	// StorageInfo returns current storage usage statistics.
	StorageInfo() (*model.StorageInfo, error)

	// SaveOffset persists the file read offset.
	SaveOffset(file string, offset int64) error

	// LoadOffset retrieves the file read offset.
	LoadOffset(file string) (int64, error)

	// Close shuts down the storage.
	Close() error
}
