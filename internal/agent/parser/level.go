package parser

import "strings"

// DetectLevel attempts to extract a log level from a log message.
// It checks common patterns: level=INFO, [INFO], INFO:, etc.
func DetectLevel(msg string) string {
	upper := strings.ToUpper(msg)

	// Check common log level keywords in order of severity
	levels := []struct {
		keywords []string
		level    string
	}{
		{[]string{"FATAL", "PANIC"}, "FATAL"},
		{[]string{"ERROR", "ERR"}, "ERROR"},
		{[]string{"WARN", "WARNING"}, "WARN"},
		{[]string{"DEBUG", "TRACE", "DBG"}, "DEBUG"},
		{[]string{"INFO", "INF"}, "INFO"},
	}

	for _, l := range levels {
		for _, kw := range l.keywords {
			idx := strings.Index(upper, kw)
			if idx < 0 {
				continue
			}
			// Verify it looks like a level marker — preceded by space, [, =, or start of line
			if idx == 0 || isSeparator(msg[idx-1]) {
				// And followed by space, ], :, or end of string
				end := idx + len(kw)
				if end >= len(msg) || isSeparator(msg[end]) || msg[end] == ':' || msg[end] == ']' {
					return l.level
				}
			}
		}
	}

	return "INFO" // default
}

func isSeparator(b byte) bool {
	return b == ' ' || b == '[' || b == '=' || b == '|' || b == '\t'
}
