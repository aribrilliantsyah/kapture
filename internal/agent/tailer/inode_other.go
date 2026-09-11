//go:build !unix

package tailer

import "os"

// Without inodes, rotation is only detected through truncation.
func inodeOf(os.FileInfo) uint64 { return 0 }
