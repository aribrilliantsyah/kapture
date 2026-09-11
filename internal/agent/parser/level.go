package parser

import (
	"regexp"
	"strings"
)

var (
	// {"level":"error"} / {"severity":"WARNING"}
	structuredLevelRe = regexp.MustCompile(`(?i)"(?:level|severity|lvl|loglevel|log\.level)"\s*:\s*"([a-z]+)`)
	// level=error / lvl="warn"
	logfmtLevelRe = regexp.MustCompile(`(?i)(?:^|[\s,{])(?:level|severity|lvl)=["']?([a-z]+)`)
	// klog header used by most Kubernetes components: E0115 10:30:00.123456 ...
	klogRe = regexp.MustCompile(`^([IWEF])\d{4} \d{2}:\d{2}:\d{2}`)
	// Terminal color codes (ESC [ ... m) that some apps write even when not on a TTY.
	ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")
)

var levelWords = map[string]string{
	"TRACE": "DEBUG", "DEBUG": "DEBUG", "DBG": "DEBUG",
	"INFO": "INFO", "INF": "INFO", "NOTICE": "INFO",
	"WARN": "WARN", "WARNING": "WARN", "WRN": "WARN",
	"ERROR": "ERROR", "ERR": "ERROR",
	"FATAL": "FATAL", "FTL": "FATAL", "PANIC": "FATAL", "CRIT": "FATAL", "CRITICAL": "FATAL", "EMERG": "FATAL",
}

var klogLevels = map[string]string{"I": "INFO", "W": "WARN", "E": "ERROR", "F": "FATAL"}

// DetectLevel extracts a normalized level (DEBUG, INFO, WARN, ERROR, FATAL)
// from a log message. Structured fields win over klog headers, which win over
// the first level-looking word near the start of the line. Defaults to INFO.
func DetectLevel(msg string) string {
	if strings.IndexByte(msg, 0x1b) >= 0 {
		msg = ansiRe.ReplaceAllString(msg, "")
	}
	if m := structuredLevelRe.FindStringSubmatch(msg); m != nil {
		if l, ok := levelWords[strings.ToUpper(m[1])]; ok {
			return l
		}
	}
	if m := logfmtLevelRe.FindStringSubmatch(msg); m != nil {
		if l, ok := levelWords[strings.ToUpper(m[1])]; ok {
			return l
		}
	}
	if m := klogRe.FindStringSubmatch(msg); m != nil {
		return klogLevels[m[1]]
	}
	head := msg
	if len(head) > 256 {
		head = head[:256]
	}
	if l, ok := firstLevelWord(head); ok {
		return l
	}
	return "INFO"
}

// firstLevelWord returns the level of the first whole word that names one.
func firstLevelWord(s string) (string, bool) {
	start := -1
	for i := 0; i <= len(s); i++ {
		if i < len(s) && isWordChar(s[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			if w := s[start:i]; len(w) >= 3 && len(w) <= 8 {
				if l, ok := levelWords[strings.ToUpper(w)]; ok {
					return l, true
				}
			}
			start = -1
		}
	}
	return "", false
}

func isWordChar(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
