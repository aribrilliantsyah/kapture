package parser

import (
	"testing"
	"time"
)

func TestParseCRI(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   CRILine
		wantOK bool
	}{
		{
			name:   "full stdout line",
			line:   "2025-01-15T10:30:00.123456789Z stdout F Hello World",
			wantOK: true,
			want: CRILine{
				Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 123456789, time.UTC),
				Stream:    "stdout",
				IsPartial: false,
				Message:   "Hello World",
			},
		},
		{
			name:   "stderr error line",
			line:   "2025-01-15T10:30:01.000000000Z stderr F ERROR: connection refused",
			wantOK: true,
			want: CRILine{
				Timestamp: time.Date(2025, 1, 15, 10, 30, 1, 0, time.UTC),
				Stream:    "stderr",
				IsPartial: false,
				Message:   "ERROR: connection refused",
			},
		},
		{
			name:   "partial line",
			line:   "2025-01-15T10:30:02.000000000Z stdout P partial message",
			wantOK: true,
			want: CRILine{
				Timestamp: time.Date(2025, 1, 15, 10, 30, 2, 0, time.UTC),
				Stream:    "stdout",
				IsPartial: true,
				Message:   "partial message",
			},
		},
		{
			name:   "empty message",
			line:   "2025-01-15T10:30:03.000000000Z stdout F ",
			wantOK: true,
			want: CRILine{
				Timestamp: time.Date(2025, 1, 15, 10, 30, 3, 0, time.UTC),
				Stream:    "stdout",
				IsPartial: false,
				Message:   "",
			},
		},
		{
			name:   "too short",
			line:   "short",
			wantOK: false,
		},
		{
			name:   "invalid timestamp",
			line:   "not-a-timestamp stdout F message",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseCRI(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ParseCRI() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if !got.Timestamp.Equal(tt.want.Timestamp) {
				t.Errorf("timestamp = %v, want %v", got.Timestamp, tt.want.Timestamp)
			}
			if got.Stream != tt.want.Stream {
				t.Errorf("stream = %q, want %q", got.Stream, tt.want.Stream)
			}
			if got.IsPartial != tt.want.IsPartial {
				t.Errorf("isPartial = %v, want %v", got.IsPartial, tt.want.IsPartial)
			}
			if got.Message != tt.want.Message {
				t.Errorf("message = %q, want %q", got.Message, tt.want.Message)
			}
		})
	}
}

func TestParseLineDocker(t *testing.T) {
	got, ok := ParseLine(`{"log":"hello world\n","stream":"stderr","time":"2025-01-15T10:30:00.5Z"}`)
	if !ok || got.Message != "hello world" || got.Stream != "stderr" || got.IsPartial ||
		!got.Timestamp.Equal(time.Date(2025, 1, 15, 10, 30, 0, 5e8, time.UTC)) {
		t.Fatalf("docker line = %+v ok=%v", got, ok)
	}
	got, ok = ParseLine(`{"log":"partial","stream":"stdout","time":"2025-01-15T10:30:00Z"}`)
	if !ok || !got.IsPartial {
		t.Fatalf("expected partial docker line, got %+v", got)
	}
	if _, ok := ParseLine(`{"not":"a log"}`); ok {
		t.Fatal("expected failure for non-log json")
	}
	if got, ok := ParseLine("2025-01-15T10:30:00Z stdout F short"); !ok || got.Message != "short" {
		t.Fatalf("short CRI line = %+v ok=%v", got, ok)
	}
}

func TestDetectLevel(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"INFO  Request handled", "INFO"},
		{"[ERROR] something failed", "ERROR"},
		{"level=WARN slow query", "WARN"},
		{"DEBUG checking cache", "DEBUG"},
		{"FATAL out of memory", "FATAL"},
		{"PANIC: runtime error", "FATAL"},
		{"ERR connection refused", "ERROR"},
		{"no level here just a message", "INFO"},
		{"WARNING: disk space low", "WARN"},
		{`{"msg":"request failed with error","level":"info"}`, "INFO"},
		{`{"severity":"ERROR","message":"x"}`, "ERROR"},
		{`ts=2025-01-15 lvl=warn msg="slow"`, "WARN"},
		{"E0115 10:30:00.123456       1 controller.go:42] sync failed", "ERROR"},
		{"W0115 10:30:00.123456       1 reflector.go:1] watch closed", "WARN"},
		{"INFO retry after error", "INFO"},
		{"2025-01-15 10:30:00 [main] error: disk full", "ERROR"},
		{"no errors found", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := DetectLevel(tt.msg)
			if got != tt.want {
				t.Errorf("DetectLevel(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}
