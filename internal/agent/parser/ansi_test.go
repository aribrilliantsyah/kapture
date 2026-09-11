package parser

import "testing"

func TestDetectLevelIgnoresColorCodes(t *testing.T) {
	cases := map[string]string{
		"\x1b[30m2026-09-11T08:03:36,828\x1b[m \x1b[1;31mERROR \x1b[m[\x1b[1;34mhttp-nio-8001-exec-10\x1b[m] boom": "ERROR",
		"\x1b[32mINFO\x1b[0m started":   "INFO",
		"\x1b[33mWARN\x1b[39m slow":     "WARN",
		"no colors, error in the words": "ERROR",
	}
	for msg, want := range cases {
		if got := DetectLevel(msg); got != want {
			t.Errorf("DetectLevel(%q) = %s, want %s", msg, got, want)
		}
	}
}
