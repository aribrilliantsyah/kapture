// Package logfilter compiles query filters once and applies them to stored
// keys (metadata first, message only when needed) and to live entries.
package logfilter

import (
	"fmt"
	"regexp"

	"github.com/aribrilliantsyah/kapture/internal/model"
	"github.com/aribrilliantsyah/kapture/internal/search"
)

// Filter is a compiled query.
type Filter struct {
	ns, wl, wtype, pod, container string
	levels                        map[string]bool
	exclude                       map[string]bool
	text                          *search.Query
	re                            *regexp.Regexp
}

// New compiles the filters of req. Field filters written in the search text
// (ns:, pod:, level: ...) fill in the ones not given as parameters.
func New(req model.QueryRequest) (*Filter, error) {
	text, err := search.Parse(req.Search)
	if err != nil {
		return nil, fmt.Errorf("invalid search: %w", err)
	}
	fl := text.Fields
	f := &Filter{
		ns:        or(req.Namespace, fl.Namespace),
		wl:        or(req.Workload, fl.Workload),
		wtype:     or(req.WorkloadType, fl.WorkloadType),
		pod:       or(req.Pod, fl.Pod),
		container: or(req.Container, fl.Container),
		text:      text,
	}
	if lv := model.SplitComma(or(req.Level, fl.Level)); len(lv) > 0 {
		f.levels = map[string]bool{}
		for _, l := range lv {
			f.levels[model.NormalizeLevel(l)] = true
		}
	}
	if len(req.ExcludeNS) > 0 {
		f.exclude = map[string]bool{}
		for _, ns := range req.ExcludeNS {
			f.exclude[ns] = true
		}
	}
	if req.Regex != "" {
		if f.re, err = regexp.Compile(req.Regex); err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	}
	return f, nil
}

// MatchMeta checks everything except the message. level must be normalized.
func (f *Filter) MatchMeta(ns, wl, wtype, pod, container, level string) bool {
	return (f.ns == "" || ns == f.ns) &&
		(f.wl == "" || wl == f.wl) &&
		(f.wtype == "" || wtype == f.wtype) &&
		(f.pod == "" || pod == f.pod) &&
		(f.container == "" || container == f.container) &&
		(f.levels == nil || f.levels[level]) &&
		!f.exclude[ns]
}

// NeedText reports whether the message must be read to decide a match.
func (f *Filter) NeedText() bool { return f.text.HasText() || f.re != nil }

// MatchText checks the message part of the query.
func (f *Filter) MatchText(msg string) bool {
	return (f.re == nil || f.re.MatchString(msg)) && f.text.Match(msg)
}

// Match checks a complete entry.
func (f *Filter) Match(e *model.LogEntry) bool {
	return f.MatchMeta(e.Namespace, e.Workload, e.WorkloadType, e.Pod, e.Container, model.NormalizeLevel(e.Level)) &&
		(!f.NeedText() || f.MatchText(e.Message))
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
