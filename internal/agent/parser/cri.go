package parser

import (
	"strings"
	"time"
)

// CRILine represents a parsed CRI log line.
type CRILine struct {
	Timestamp time.Time
	Stream    string // stdout | stderr
	IsPartial bool
	Message   string
}

// ParseCRI parses a CRI-format log line:
// "2025-01-15T10:30:00.123456789Z stdout F message here"
// "2025-01-15T10:30:00.123456789Z stderr P partial mess"
func ParseCRI(line string) (CRILine, bool) {
	// Minimum: "2006-01-02T15:04:05Z stdout F x" = ~35 chars
	if len(line) < 35 {
		return CRILine{}, false
	}

	// Find first space → end of timestamp
	sp1 := strings.IndexByte(line, ' ')
	if sp1 < 0 {
		return CRILine{}, false
	}
	tsStr := line[:sp1]
	rest := line[sp1+1:]

	// Parse timestamp
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		return CRILine{}, false
	}

	// Find second space → end of stream
	sp2 := strings.IndexByte(rest, ' ')
	if sp2 < 0 {
		return CRILine{}, false
	}
	stream := rest[:sp2]
	rest = rest[sp2+1:]

	// Find third space → end of flags
	sp3 := strings.IndexByte(rest, ' ')
	if sp3 < 0 {
		// No message, just flag
		return CRILine{
			Timestamp: ts,
			Stream:    stream,
			IsPartial: rest == "P",
			Message:   "",
		}, true
	}
	flags := rest[:sp3]
	msg := rest[sp3+1:]

	return CRILine{
		Timestamp: ts,
		Stream:    stream,
		IsPartial: flags == "P",
		Message:   msg,
	}, true
}
