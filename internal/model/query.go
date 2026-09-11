package model

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QueryRequest holds filter parameters for querying logs.
type QueryRequest struct {
	Date         string     `json:"date"` // YYYY-MM-DD (UTC)
	From         *time.Time `json:"from"`
	To           *time.Time `json:"to"`
	Namespace    string     `json:"namespace"`
	Workload     string     `json:"workload"`
	WorkloadType string     `json:"workload_type"`
	Pod          string     `json:"pod"`
	Container    string     `json:"container"`
	Level        string     `json:"level"` // comma separated
	Search       string     `json:"search"`
	Regex        string     `json:"regex"`
	ExcludeNS    []string   `json:"exclude_ns"`
	Limit        int        `json:"limit"`
	Cursor       string     `json:"cursor"` // unix nano, exclusive bound in sort direction
	Sort         string     `json:"sort"`   // asc / desc
	Buckets      int        `json:"buckets"`
}

// ParseQuery builds a QueryRequest from URL query parameters.
func ParseQuery(q url.Values) QueryRequest {
	req := QueryRequest{
		Date:         q.Get("date"),
		Namespace:    q.Get("namespace"),
		Workload:     q.Get("workload"),
		WorkloadType: q.Get("workload_type"),
		Pod:          q.Get("pod"),
		Container:    q.Get("container"),
		Level:        q.Get("level"),
		Search:       q.Get("search"),
		Regex:        q.Get("regex"),
		Cursor:       q.Get("cursor"),
		Sort:         q.Get("sort"),
		ExcludeNS:    SplitComma(q.Get("exclude_ns")),
	}
	req.Limit, _ = strconv.Atoi(q.Get("limit"))
	req.Buckets, _ = strconv.Atoi(q.Get("buckets"))
	req.From = parseTime(q.Get("from"))
	req.To = parseTime(q.Get("to"))
	return req
}

// Values encodes the request back into URL query parameters.
func (r QueryRequest) Values() url.Values {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("date", r.Date)
	set("namespace", r.Namespace)
	set("workload", r.Workload)
	set("workload_type", r.WorkloadType)
	set("pod", r.Pod)
	set("container", r.Container)
	set("level", r.Level)
	set("search", r.Search)
	set("regex", r.Regex)
	set("cursor", r.Cursor)
	set("sort", r.Sort)
	set("exclude_ns", strings.Join(r.ExcludeNS, ","))
	if r.Limit > 0 {
		q.Set("limit", strconv.Itoa(r.Limit))
	}
	if r.Buckets > 0 {
		q.Set("buckets", strconv.Itoa(r.Buckets))
	}
	if r.From != nil {
		q.Set("from", r.From.UTC().Format(time.RFC3339Nano))
	}
	if r.To != nil {
		q.Set("to", r.To.UTC().Format(time.RFC3339Nano))
	}
	return q
}

// Desc reports whether results are ordered newest first.
func (r QueryRequest) Desc() bool { return r.Sort != "asc" }

// SplitComma splits a comma separated list, dropping empty items.
func SplitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseTime accepts RFC3339(Nano) or a unix-nano integer.
func parseTime(v string) *time.Time {
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return &t
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		t := time.Unix(0, n).UTC()
		return &t
	}
	return nil
}
