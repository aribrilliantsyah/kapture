package fanout

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/aggregator/discovery"
	"github.com/ordinary/k8s-log-catcher/internal/model"
)

// Client fans requests out to every agent and merges the replies.
type Client struct {
	discovery discovery.Provider
	http      *http.Client
}

// New creates a new fanout client.
func New(disc discovery.Provider, timeout time.Duration) *Client {
	return &Client{discovery: disc, http: &http.Client{Timeout: timeout}}
}

// StatusError is an error reply from an agent.
type StatusError struct {
	Code int
	Msg  string
}

func (e *StatusError) Error() string { return e.Msg }

type reply[T any] struct {
	endpoint string
	val      T
	err      error
}

// all sends the same request to every agent in parallel.
func all[T any](c *Client, method, path string, q url.Values) []reply[T] {
	eps := c.discovery.Endpoints()
	out := make([]reply[T], len(eps))
	var wg sync.WaitGroup
	for i, ep := range eps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i].endpoint = ep
			out[i].err = c.do(method, ep, path, q, &out[i].val)
		}()
	}
	wg.Wait()
	return out
}

func (c *Client) do(method, endpoint, path string, q url.Values, out any) error {
	u := endpoint + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequest(method, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("agent %s unreachable: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) != nil || e.Error == "" {
			e.Error = fmt.Sprintf("agent %s: HTTP %d", endpoint, resp.StatusCode)
		}
		return &StatusError{Code: resp.StatusCode, Msg: e.Error}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("agent %s: bad response: %w", endpoint, err)
	}
	return nil
}

// collectErrors records failed agents. If every agent rejected the request
// (e.g. an invalid regex) that rejection is returned instead.
func collectErrors[T any](replies []reply[T], errs *[]string) error {
	if len(replies) == 0 {
		*errs = append(*errs, "no agents discovered")
		return nil
	}
	ok := 0
	var rejected error
	for _, r := range replies {
		if r.err == nil {
			ok++
			continue
		}
		var se *StatusError
		if errors.As(r.err, &se) && se.Code < 500 {
			rejected = se
		}
		*errs = append(*errs, r.err.Error())
	}
	if ok == 0 && rejected != nil {
		return rejected
	}
	return nil
}

// QueryLogs queries every agent and merges the pages by timestamp.
//
// Each agent that has more results reports the timestamp it stopped at. The
// merged page may not reach past the furthest of those bounds, otherwise the
// next page (which starts after the last shown entry) would skip that agent's
// unseen entries.
func (c *Client) QueryLogs(req model.QueryRequest) (*model.QueryResult, error) {
	replies := all[model.QueryResult](c, http.MethodGet, "/api/v1/logs", req.Values())
	res := &model.QueryResult{Entries: []model.LogEntry{}}
	if err := collectErrors(replies, &res.Errors); err != nil {
		return nil, err
	}

	desc := req.Desc()
	var bound int64
	bounded := false
	for _, r := range replies {
		if r.err != nil {
			continue
		}
		res.Entries = append(res.Entries, r.val.Entries...)
		res.Partial = res.Partial || r.val.Partial
		if n, err := strconv.ParseInt(r.val.NextCursor, 10, 64); err == nil {
			if !bounded || (desc && n > bound) || (!desc && n < bound) {
				bound, bounded = n, true
			}
		}
	}
	if bounded {
		res.Entries = slices.DeleteFunc(res.Entries, func(e model.LogEntry) bool {
			ts := e.Timestamp.UnixNano()
			return (desc && ts < bound) || (!desc && ts > bound)
		})
	}
	sortEntries(res.Entries, desc)

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	more := bounded
	if len(res.Entries) > limit {
		res.Entries = res.Entries[:limit]
		more = true
	}
	if more {
		if n := len(res.Entries); n > 0 {
			res.NextCursor = strconv.FormatInt(res.Entries[n-1].Timestamp.UnixNano(), 10)
		} else {
			res.NextCursor = strconv.FormatInt(bound, 10)
		}
	}
	return res, nil
}

func sortEntries(entries []model.LogEntry, desc bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.Timestamp.Equal(b.Timestamp) {
			return a.Timestamp.After(b.Timestamp) == desc
		}
		if a.Pod != b.Pod {
			return (a.Pod > b.Pod) == desc
		}
		return (a.Seq > b.Seq) == desc
	})
}

// Volume merges the log-volume histograms of all agents.
func (c *Client) Volume(req model.QueryRequest) (*model.VolumeResult, error) {
	// Pin the window so every agent uses the same bucket boundaries.
	now := time.Now().UTC()
	if req.To == nil {
		req.To = &now
	}
	if req.From == nil {
		from := req.To.Add(-24 * time.Hour)
		req.From = &from
	}
	replies := all[model.VolumeResult](c, http.MethodGet, "/api/v1/stats/volume", req.Values())
	res := &model.VolumeResult{
		From: req.From.UnixNano(), To: req.To.UnixNano(),
		Buckets: []model.VolumeBucket{}, Totals: map[string]int64{},
	}
	if err := collectErrors(replies, &res.Errors); err != nil {
		return nil, err
	}

	top := map[[2]string]int64{}
	for _, r := range replies {
		if r.err != nil {
			continue
		}
		v := r.val
		if len(res.Buckets) == 0 {
			res.BucketNanos, res.Buckets = v.BucketNanos, v.Buckets
		} else {
			for i, b := range v.Buckets {
				if i < len(res.Buckets) {
					for l, n := range b.Counts {
						res.Buckets[i].Counts[l] += n
					}
				}
			}
		}
		for l, n := range v.Totals {
			res.Totals[l] += n
		}
		for _, t := range v.TopErrors {
			top[[2]string{t.Namespace, t.Workload}] += t.Count
		}
		res.Partial = res.Partial || v.Partial
	}

	res.TopErrors = make([]model.WorkloadCount, 0, len(top))
	for k, n := range top {
		res.TopErrors = append(res.TopErrors, model.WorkloadCount{Namespace: k[0], Workload: k[1], Count: n})
	}
	sort.Slice(res.TopErrors, func(i, j int) bool { return res.TopErrors[i].Count > res.TopErrors[j].Count })
	if len(res.TopErrors) > 10 {
		res.TopErrors = res.TopErrors[:10]
	}
	return res, nil
}

// Catalog concatenates the container catalogs of all agents.
func (c *Client) Catalog() ([]model.CatalogItem, []string) {
	replies := all[[]model.CatalogItem](c, http.MethodGet, "/api/v1/catalog", nil)
	var errs []string
	_ = collectErrors(replies, &errs)
	items := []model.CatalogItem{}
	for _, r := range replies {
		if r.err == nil {
			items = append(items, r.val...)
		}
	}
	return items, errs
}

// GetDates returns the union of dates with logs, newest first.
func (c *Client) GetDates() []string {
	d := c.collectStrings("/api/v1/dates", nil)
	slices.Reverse(d)
	return d
}

// GetNamespaces returns the union of namespaces.
func (c *Client) GetNamespaces() []string {
	return c.collectStrings("/api/v1/namespaces", nil)
}

// GetWorkloads returns the union of workloads.
func (c *Client) GetWorkloads(namespace string) []string {
	return c.collectStrings("/api/v1/workloads", url.Values{"namespace": {namespace}})
}

// GetPods returns the union of pods.
func (c *Client) GetPods(namespace, workload string) []string {
	return c.collectStrings("/api/v1/pods", url.Values{"namespace": {namespace}, "workload": {workload}})
}

func (c *Client) collectStrings(path string, q url.Values) []string {
	seen := map[string]bool{}
	for _, r := range all[[]string](c, http.MethodGet, path, q) {
		for _, v := range r.val {
			seen[v] = true
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// GetStorageInfo sums the storage usage of all agents.
func (c *Client) GetStorageInfo() *model.StorageInfo {
	agg := &model.StorageInfo{Dates: []model.DateBreakdown{}, Nodes: []model.StorageInfo{}}
	dates := map[string]*model.DateBreakdown{}
	for _, r := range all[model.StorageInfo](c, http.MethodGet, "/api/v1/storage", nil) {
		if r.err != nil {
			agg.Nodes = append(agg.Nodes, model.StorageInfo{Node: r.endpoint + " (unreachable)"})
			continue
		}
		info := r.val
		agg.UsedBytes += info.UsedBytes
		agg.MaxBytes += info.MaxBytes
		agg.EntryCount += info.EntryCount
		agg.Retention = info.Retention
		if agg.Timezone == "" {
			agg.Timezone = info.Timezone
		}
		if info.OldestDate != "" && (agg.OldestDate == "" || info.OldestDate < agg.OldestDate) {
			agg.OldestDate = info.OldestDate
		}
		if info.NewestDate > agg.NewestDate {
			agg.NewestDate = info.NewestDate
		}
		for _, d := range info.Dates {
			if cur, ok := dates[d.Date]; ok {
				cur.EntryCount += d.EntryCount
				cur.SizeBytes += d.SizeBytes
			} else {
				d := d
				dates[d.Date] = &d
			}
		}
		info.Dates = nil
		agg.Nodes = append(agg.Nodes, info)
	}
	for _, d := range dates {
		agg.Dates = append(agg.Dates, *d)
	}
	sort.Slice(agg.Dates, func(i, j int) bool { return agg.Dates[i].Date > agg.Dates[j].Date })
	sort.Slice(agg.Nodes, func(i, j int) bool { return agg.Nodes[i].Node < agg.Nodes[j].Node })
	return agg
}

// DeleteLogs fans a delete out to all agents and returns the total deleted.
func (c *Client) DeleteLogs(req model.DeleteRequest) (int64, []string) {
	path, q := "/api/v1/logs", url.Values{}
	if req.All {
		path += "/all"
	} else {
		for k, v := range map[string]string{"date": req.Date, "before": req.BeforeDate, "namespace": req.Namespace, "workload": req.Workload} {
			if v != "" {
				q.Set(k, v)
			}
		}
	}
	replies := all[struct {
		Deleted int64 `json:"deleted"`
	}](c, http.MethodDelete, path, q)
	var total int64
	var errs []string
	_ = collectErrors(replies, &errs)
	for _, r := range replies {
		total += r.val.Deleted
	}
	return total, errs
}

// AgentStatus is the health of one agent.
type AgentStatus struct {
	Endpoint string `json:"endpoint"`
	Node     string `json:"node,omitempty"`
	Version  string `json:"version,omitempty"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// Agents checks every discovered agent.
func (c *Client) Agents() []AgentStatus {
	type health struct {
		Status  string `json:"status"`
		Node    string `json:"node"`
		Version string `json:"version"`
	}
	replies := all[health](c, http.MethodGet, "/healthz", nil)
	out := make([]AgentStatus, 0, len(replies))
	for _, r := range replies {
		s := AgentStatus{Endpoint: r.endpoint, Node: r.val.Node, Version: r.val.Version, Status: "ok"}
		if r.err != nil {
			s.Status, s.Error = "unreachable", r.err.Error()
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Node+out[i].Endpoint < out[j].Node+out[j].Endpoint })
	return out
}
