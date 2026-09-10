package fanout

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/aggregator/discovery"
	"github.com/ordinary/k8s-log-catcher/internal/model"
)

// Client fans out queries to multiple agents and merges results.
type Client struct {
	discovery discovery.Provider
	http      *http.Client
}

// New creates a new fanout client.
func New(disc discovery.Provider, timeout time.Duration) *Client {
	return &Client{
		discovery: disc,
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

// QueryLogs fans out a log query to all agents and merges results by timestamp.
func (c *Client) QueryLogs(req model.QueryRequest) (*model.QueryResult, error) {
	endpoints := c.discovery.Endpoints()
	if len(endpoints) == 0 {
		return &model.QueryResult{}, nil
	}

	type agentResult struct {
		result *model.QueryResult
		err    error
	}

	results := make([]agentResult, len(endpoints))
	var wg sync.WaitGroup

	for i, ep := range endpoints {
		wg.Add(1)
		go func(idx int, endpoint string) {
			defer wg.Done()
			qr, err := c.queryAgent(endpoint, req)
			results[idx] = agentResult{result: qr, err: err}
		}(i, ep)
	}

	wg.Wait()

	// Merge all results
	merged := &model.QueryResult{}
	for _, r := range results {
		if r.err != nil {
			slog.Warn("agent query failed", "error", r.err)
			continue
		}
		if r.result != nil {
			merged.Entries = append(merged.Entries, r.result.Entries...)
		}
	}

	// Sort by timestamp
	sort.Slice(merged.Entries, func(i, j int) bool {
		if req.Sort == "desc" {
			return merged.Entries[i].Timestamp.After(merged.Entries[j].Timestamp)
		}
		return merged.Entries[i].Timestamp.Before(merged.Entries[j].Timestamp)
	})

	// Apply limit
	limit := req.Limit
	if limit <= 0 || limit > 10000 {
		limit = 100
	}
	if len(merged.Entries) > limit {
		merged.Entries = merged.Entries[:limit]
	}

	return merged, nil
}

// queryAgent queries a single agent endpoint.
func (c *Client) queryAgent(endpoint string, req model.QueryRequest) (*model.QueryResult, error) {
	u, err := url.Parse(endpoint + "/api/v1/logs")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if req.Date != "" {
		q.Set("date", req.Date)
	}
	if req.Namespace != "" {
		q.Set("namespace", req.Namespace)
	}
	if req.Workload != "" {
		q.Set("workload", req.Workload)
	}
	if req.WorkloadType != "" {
		q.Set("workload_type", req.WorkloadType)
	}
	if req.Pod != "" {
		q.Set("pod", req.Pod)
	}
	if req.Container != "" {
		q.Set("container", req.Container)
	}
	if req.Level != "" {
		q.Set("level", req.Level)
	}
	if req.Search != "" {
		q.Set("search", req.Search)
	}
	if req.Regex != "" {
		q.Set("regex", req.Regex)
	}
	if req.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", req.Limit))
	}
	if req.Sort != "" {
		q.Set("sort", req.Sort)
	}
	if req.From != nil {
		q.Set("from", req.From.Format(time.RFC3339))
	}
	if req.To != nil {
		q.Set("to", req.To.Format(time.RFC3339))
	}
	u.RawQuery = q.Encode()

	resp, err := c.http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result model.QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response from %s: %w", endpoint, err)
	}

	return &result, nil
}

// GetDates fans out and collects unique dates from all agents.
func (c *Client) GetDates() ([]string, error) {
	return c.collectStrings("/api/v1/dates")
}

// GetNamespaces fans out and collects unique namespaces.
func (c *Client) GetNamespaces() ([]string, error) {
	return c.collectStrings("/api/v1/namespaces")
}

// GetWorkloads fans out and collects unique workloads.
func (c *Client) GetWorkloads(namespace string) ([]string, error) {
	path := "/api/v1/workloads"
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}
	return c.collectStrings(path)
}

// GetPods fans out and collects unique pods.
func (c *Client) GetPods(namespace, workload string) ([]string, error) {
	q := url.Values{}
	if namespace != "" {
		q.Set("namespace", namespace)
	}
	if workload != "" {
		q.Set("workload", workload)
	}
	path := "/api/v1/pods"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return c.collectStrings(path)
}

// GetStorageInfo collects and aggregates storage info from all agents.
func (c *Client) GetStorageInfo() (*model.StorageInfo, error) {
	endpoints := c.discovery.Endpoints()
	aggregated := &model.StorageInfo{}
	dateMap := make(map[string]*model.DateBreakdown)

	for _, ep := range endpoints {
		info, err := c.getAgentStorage(ep)
		if err != nil {
			slog.Warn("failed to get storage info", "endpoint", ep, "error", err)
			continue
		}
		aggregated.UsedBytes += info.UsedBytes
		aggregated.MaxBytes += info.MaxBytes
		aggregated.EntryCount += info.EntryCount

		if aggregated.OldestDate == "" || (info.OldestDate != "" && info.OldestDate < aggregated.OldestDate) {
			aggregated.OldestDate = info.OldestDate
		}
		if info.NewestDate > aggregated.NewestDate {
			aggregated.NewestDate = info.NewestDate
		}

		for _, d := range info.Dates {
			if existing, ok := dateMap[d.Date]; ok {
				existing.EntryCount += d.EntryCount
				existing.SizeBytes += d.SizeBytes
			} else {
				copy := d
				dateMap[d.Date] = &copy
			}
		}
	}

	aggregated.Dates = make([]model.DateBreakdown, 0, len(dateMap))
	for _, d := range dateMap {
		aggregated.Dates = append(aggregated.Dates, *d)
	}
	sort.Slice(aggregated.Dates, func(i, j int) bool {
		return aggregated.Dates[i].Date > aggregated.Dates[j].Date
	})

	return aggregated, nil
}

func (c *Client) getAgentStorage(endpoint string) (*model.StorageInfo, error) {
	resp, err := c.http.Get(endpoint + "/api/v1/storage")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info model.StorageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// DeleteLogs fans out a delete request to all agents.
func (c *Client) DeleteLogs(req model.DeleteRequest) (int64, error) {
	endpoints := c.discovery.Endpoints()
	var totalDeleted int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ep := range endpoints {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			deleted, err := c.deleteFromAgent(endpoint, req)
			if err != nil {
				slog.Warn("delete failed on agent", "endpoint", endpoint, "error", err)
				return
			}
			mu.Lock()
			totalDeleted += deleted
			mu.Unlock()
		}(ep)
	}

	wg.Wait()
	return totalDeleted, nil
}

func (c *Client) deleteFromAgent(endpoint string, req model.DeleteRequest) (int64, error) {
	u := endpoint + "/api/v1/logs"
	if req.All {
		u = endpoint + "/api/v1/logs/all"
	} else {
		q := url.Values{}
		if req.BeforeDate != "" {
			q.Set("before", req.BeforeDate)
		}
		if req.Namespace != "" {
			q.Set("namespace", req.Namespace)
		}
		if req.Workload != "" {
			q.Set("workload", req.Workload)
		}
		if len(q) > 0 {
			u += "?" + q.Encode()
		}
	}

	httpReq, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Deleted int64  `json:"deleted"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Deleted, nil
}

// collectStrings fans out a GET request to all agents and merges string arrays.
func (c *Client) collectStrings(path string) ([]string, error) {
	endpoints := c.discovery.Endpoints()
	unique := make(map[string]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ep := range endpoints {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			resp, err := c.http.Get(endpoint + path)
			if err != nil {
				slog.Warn("request failed", "endpoint", endpoint, "path", path, "error", err)
				return
			}
			defer resp.Body.Close()

			var items []string
			if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
				return
			}

			mu.Lock()
			for _, item := range items {
				unique[item] = true
			}
			mu.Unlock()
		}(ep)
	}

	wg.Wait()

	result := make([]string, 0, len(unique))
	for v := range unique {
		result = append(result, v)
	}
	sort.Strings(result)
	return result, nil
}

// HealthCheck checks connectivity to all agents.
func (c *Client) HealthCheck() map[string]string {
	endpoints := c.discovery.Endpoints()
	status := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ep := range endpoints {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			resp, err := c.http.Get(endpoint + "/healthz")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				status[endpoint] = "unreachable"
				return
			}
			resp.Body.Close()
			if resp.StatusCode == 200 {
				status[endpoint] = "ok"
			} else {
				status[endpoint] = fmt.Sprintf("http %d", resp.StatusCode)
			}
		}(ep)
	}

	wg.Wait()
	return status
}
