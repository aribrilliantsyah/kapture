package parser

import (
	"encoding/json"
	"strings"
	"time"
)

// CRILine is one physical line of a container log file.
type CRILine struct {
	Timestamp time.Time
	Stream    string // stdout | stderr
	IsPartial bool
	Message   string
}

// ParseLine parses a container log line written by containerd / CRI-O (CRI
// format) or by Docker's json-file driver (dockershim, cri-dockerd).
func ParseLine(line string) (CRILine, bool) {
	if strings.HasPrefix(line, "{") {
		return parseDockerJSON(line)
	}
	return ParseCRI(line)
}

// ParseCRI parses a CRI-format log line:
//
//	2025-01-15T10:30:00.123456789Z stdout F message here
//	2025-01-15T10:30:00.123456789Z stderr P partial mess
func ParseCRI(line string) (CRILine, bool) {
	tsStr, rest, ok := strings.Cut(line, " ")
	if !ok {
		return CRILine{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		return CRILine{}, false
	}
	stream, rest, ok := strings.Cut(rest, " ")
	if !ok {
		return CRILine{}, false
	}
	flags, msg, _ := strings.Cut(rest, " ")
	return CRILine{
		Timestamp: ts.UTC(),
		Stream:    stream,
		IsPartial: flags == "P" || strings.HasPrefix(flags, "P:"),
		Message:   msg,
	}, true
}

type dockerLine struct {
	Log    string    `json:"log"`
	Stream string    `json:"stream"`
	Time   time.Time `json:"time"`
}

// parseDockerJSON parses {"log":"msg\n","stream":"stdout","time":"..."}.
// A log value without a trailing newline is a partial line.
func parseDockerJSON(line string) (CRILine, bool) {
	var d dockerLine
	if err := json.Unmarshal([]byte(line), &d); err != nil || d.Time.IsZero() {
		return CRILine{}, false
	}
	msg, complete := strings.CutSuffix(d.Log, "\n")
	return CRILine{
		Timestamp: d.Time.UTC(),
		Stream:    d.Stream,
		IsPartial: !complete,
		Message:   strings.TrimSuffix(msg, "\r"),
	}, true
}
