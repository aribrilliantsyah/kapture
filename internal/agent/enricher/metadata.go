package enricher

import (
	"path/filepath"
	"strings"
)

// FileMeta holds metadata extracted from a Kubernetes container log filename.
type FileMeta struct {
	Pod       string
	Namespace string
	Container string
}

// ParseFilename extracts pod, namespace, and container from a K8s container log filename.
// Format: <pod_name>_<namespace>_<container_name>-<container_id>.log
func ParseFilename(path string) (FileMeta, bool) {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".log")

	// Split by underscore: pod_namespace_container-id
	parts := strings.SplitN(base, "_", 3)
	if len(parts) < 3 {
		return FileMeta{}, false
	}

	pod := parts[0]
	namespace := parts[1]
	containerWithID := parts[2]

	// Container name is everything before the last dash+64-char hex ID
	// e.g. "app-a1b2c3d4e5f6..." → "app"
	container := extractContainerName(containerWithID)

	return FileMeta{
		Pod:       pod,
		Namespace: namespace,
		Container: container,
	}, true
}

// extractContainerName strips the container ID suffix.
// Input:  "my-container-a1b2c3d4e5f6abcd1234567890abcdef1234567890abcdef12345678"
// Output: "my-container"
func extractContainerName(s string) string {
	// The container ID is a 64-char hex string after the last dash.
	// But some container names have dashes, so we find the last dash
	// where the remaining part is exactly 64 hex chars.
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' {
			suffix := s[i+1:]
			if len(suffix) == 64 && isHex(suffix) {
				return s[:i]
			}
		}
	}
	// Fallback: just return as-is
	return s
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
