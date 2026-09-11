// Package search implements the dashboard query syntax:
//
//	timeout database          AND (default)
//	error OR warning          OR
//	error -healthcheck        NOT
//	"connection refused"      exact phrase
//	level:error pod:api-0     field filters (namespace, workload, pod, container, level, type)
//	/failed.*\d+ retries/     regex
//
// Text matching is case-insensitive.
package search

import (
	"regexp"
	"strings"
)

// Fields holds field filters extracted from the query text.
type Fields struct {
	Namespace    string
	Workload     string
	WorkloadType string
	Pod          string
	Container    string
	Level        string
}

type term struct {
	text   string // lower-cased
	negate bool
}

// Query is a compiled search expression.
type Query struct {
	groups [][]term // OR of AND-groups
	re     *regexp.Regexp
	Fields Fields
}

// Parse compiles a search string. It returns nil when nothing needs matching.
func Parse(s string) (*Query, error) {
	q := &Query{}
	group := []term{}
	for _, tok := range tokenize(s) {
		switch {
		case tok == "OR":
			if len(group) > 0 {
				q.groups = append(q.groups, group)
				group = []term{}
			}
		case len(tok) > 2 && tok[0] == '/' && tok[len(tok)-1] == '/':
			re, err := regexp.Compile("(?i)" + tok[1:len(tok)-1])
			if err != nil {
				return nil, err
			}
			q.re = re
		case q.field(tok):
		default:
			t := term{}
			if tok[0] == '-' && len(tok) > 1 {
				t.negate = true
				tok = tok[1:]
			}
			t.text = strings.ToLower(strings.Trim(tok, `"`))
			if t.text != "" {
				group = append(group, t)
			}
		}
	}
	if len(group) > 0 {
		q.groups = append(q.groups, group)
	}
	return q, nil
}

// field consumes a key:value token.
func (q *Query) field(tok string) bool {
	key, val, ok := strings.Cut(tok, ":")
	if !ok || val == "" || strings.HasPrefix(tok, `"`) {
		return false
	}
	val = strings.Trim(val, `"`)
	switch strings.ToLower(key) {
	case "namespace", "ns":
		q.Fields.Namespace = val
	case "workload", "wl":
		q.Fields.Workload = val
	case "type":
		q.Fields.WorkloadType = strings.ToLower(val)
	case "pod":
		q.Fields.Pod = val
	case "container", "c":
		q.Fields.Container = val
	case "level", "lvl":
		q.Fields.Level = strings.ToUpper(val)
	default:
		return false
	}
	return true
}

// HasText reports whether the query needs the message body to decide a match.
func (q *Query) HasText() bool { return q != nil && (len(q.groups) > 0 || q.re != nil) }

// Match reports whether msg satisfies the text part of the query.
func (q *Query) Match(msg string) bool {
	if !q.HasText() {
		return true
	}
	if q.re != nil && !q.re.MatchString(msg) {
		return false
	}
	if len(q.groups) == 0 {
		return true
	}
	lower := strings.ToLower(msg)
	for _, g := range q.groups {
		if matchGroup(lower, g) {
			return true
		}
	}
	return false
}

func matchGroup(lower string, g []term) bool {
	for _, t := range g {
		if strings.Contains(lower, t.text) == t.negate {
			return false
		}
	}
	return true
}

// tokenize splits on whitespace, keeping "quoted phrases" and /regex/ intact.
func tokenize(s string) []string {
	var toks []string
	var cur strings.Builder
	inQuote, inRegex := false, false
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' && !inRegex:
			inQuote = !inQuote
			cur.WriteByte(c)
		case c == '/' && !inQuote && (cur.Len() == 0 || inRegex):
			cur.WriteByte(c)
			if inRegex {
				inRegex = false
				flush()
			} else {
				inRegex = true
			}
		case (c == ' ' || c == '\t') && !inQuote && !inRegex:
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return toks
}
