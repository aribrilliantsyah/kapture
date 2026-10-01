package fanout

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/aribrilliantsyah/kapture/internal/model"
)

const (
	tailRediscover = 10 * time.Second
	tailRetry      = 2 * time.Second
)

// streamClient has no overall timeout: tail streams stay open indefinitely.
var streamClient = &http.Client{}

// Tail follows the live stream of every agent and forwards entries to out
// until ctx ends. Agents that appear later (new nodes) are picked up, agents
// that disappear are dropped, broken streams are reconnected.
func (c *Client) Tail(ctx context.Context, req model.QueryRequest, out chan<- model.LogEntry) {
	q := req.Values()
	q.Del("cursor")
	active := map[string]context.CancelFunc{}
	sync := func() {
		seen := map[string]bool{}
		for _, ep := range c.discovery.Endpoints() {
			seen[ep] = true
			if _, ok := active[ep]; !ok {
				cctx, cancel := context.WithCancel(ctx)
				active[ep] = cancel
				go follow(cctx, ep, q, out)
			}
		}
		for ep, cancel := range active {
			if !seen[ep] {
				cancel()
				delete(active, ep)
			}
		}
	}
	sync()
	ticker := time.NewTicker(tailRediscover)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			for _, cancel := range active {
				cancel()
			}
			return
		case <-ticker.C:
			sync()
		}
	}
}

func follow(ctx context.Context, endpoint string, q url.Values, out chan<- model.LogEntry) {
	for ctx.Err() == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/api/v1/tail?"+q.Encode(), nil)
		if err != nil {
			return
		}
		if resp, err := streamClient.Do(req); err == nil {
			if resp.StatusCode == http.StatusOK {
				sc := bufio.NewScanner(resp.Body)
				sc.Buffer(make([]byte, 64<<10), 4<<20)
				for sc.Scan() {
					line := sc.Bytes()
					if len(line) == 0 {
						continue // keep-alive
					}
					var e model.LogEntry
					if json.Unmarshal(line, &e) != nil {
						continue
					}
					select {
					case out <- e:
					case <-ctx.Done():
						resp.Body.Close()
						return
					}
				}
			}
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(tailRetry):
		}
	}
}

// SortAsc orders entries oldest first.
func SortAsc(entries []model.LogEntry) { sortEntries(entries, false) }

// Recap merges the daily workload rollups of all agents.
func (c *Client) Recap(from, to, namespace string) ([]model.WorkloadDay, []string) {
	q := url.Values{}
	for k, v := range map[string]string{"from": from, "to": to, "namespace": namespace} {
		if v != "" {
			q.Set(k, v)
		}
	}
	replies := all[[]model.WorkloadDay](c, http.MethodGet, "/api/v1/stats/recap", q)
	var errs []string
	_ = collectErrors(replies, &errs)

	type key struct{ date, ns, wl string }
	merged := map[key]*model.WorkloadDay{}
	for _, r := range replies {
		for _, d := range r.val {
			k := key{d.Date, d.Namespace, d.Workload}
			m := merged[k]
			if m == nil {
				d := d
				if d.Levels == nil {
					d.Levels = map[string]int64{}
				}
				merged[k] = &d
				continue
			}
			for l, n := range d.Levels {
				m.Levels[l] += n
			}
			m.Bytes += d.Bytes
			m.First = min(m.First, d.First)
			m.Last = max(m.Last, d.Last)
			for _, p := range d.Pods {
				if !contains(m.Pods, p) {
					m.Pods = append(m.Pods, p)
				}
			}
		}
	}
	out := make([]model.WorkloadDay, 0, len(merged))
	for _, d := range merged {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Namespace+"/"+out[i].Workload < out[j].Namespace+"/"+out[j].Workload
	})
	return out, errs
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
