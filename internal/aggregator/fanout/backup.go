package fanout

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/version"
)

// A backup archive (.tar.gz) holds, per agent, its Badger backup stream split
// into parts, then a manifest:
//
//	nodes/<node>/000001.badger ...
//	manifest.json
//
// Parts exist because a tar entry needs its size up front while agents
// stream their backup without one; each part is buffered in memory.
const (
	backupFormat  = "kapture-backup"
	backupPart    = 8 << 20
	manifestEntry = "manifest.json"
)

// BackupManifest describes a backup archive.
type BackupManifest struct {
	Format    string       `json:"format"`
	Version   string       `json:"version"`
	CreatedAt time.Time    `json:"created_at"`
	Timezone  string       `json:"timezone"`
	From      string       `json:"from,omitempty"`
	To        string       `json:"to,omitempty"`
	Nodes     []BackupNode `json:"nodes"`
}

// BackupNode is the backup of one agent.
type BackupNode struct {
	Node     string `json:"node"`
	Endpoint string `json:"endpoint"`
	Entries  int64  `json:"entries"`
	Bytes    int64  `json:"bytes"`
	Error    string `json:"error,omitempty"`
}

// RestoreResult is the restore of one node's logs.
type RestoreResult struct {
	Node    string `json:"node"`
	Target  string `json:"target"` // node of the agent that received the logs
	Entries int64  `json:"entries"`
	Error   string `json:"error,omitempty"`
}

// Endpoints returns the currently discovered agents.
func (c *Client) Endpoints() []string { return c.discovery.Endpoints() }

// Backup writes the archive to w, one agent after the other.
func (c *Client) Backup(ctx context.Context, w io.Writer, from, to, tz string) (*BackupManifest, error) {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	man := &BackupManifest{Format: backupFormat, Version: version.Version, CreatedAt: time.Now().UTC(), Timezone: tz, From: from, To: to}

	eps := c.Endpoints()
	sort.Strings(eps)
	used := map[string]bool{}
	buf := make([]byte, backupPart)
	for _, ep := range eps {
		man.Nodes = append(man.Nodes, backupAgent(ctx, tw, buf, ep, from, to, used))
		if err := ctx.Err(); err != nil {
			return man, err
		}
	}

	b, _ := json.MarshalIndent(man, "", "  ")
	if err := tw.WriteHeader(&tar.Header{Name: manifestEntry, Mode: 0o644, Size: int64(len(b)), ModTime: man.CreatedAt}); err != nil {
		return man, err
	}
	if _, err := tw.Write(b); err != nil {
		return man, err
	}
	if err := tw.Close(); err != nil {
		return man, err
	}
	return man, gz.Close()
}

func backupAgent(ctx context.Context, tw *tar.Writer, buf []byte, ep, from, to string, used map[string]bool) BackupNode {
	bn := BackupNode{Endpoint: ep, Node: hostOf(ep)}
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep+"/api/v1/backup?"+q.Encode(), nil)
	if err != nil {
		bn.Error = err.Error()
		return bn
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		bn.Error = fmt.Sprintf("agent unreachable: %v", err)
		return bn
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bn.Error = fmt.Sprintf("agent replied HTTP %d", resp.StatusCode)
		return bn
	}
	if n := resp.Header.Get("X-Kapture-Node"); n != "" {
		bn.Node = n
	}
	dir := safeName(bn.Node)
	for i := 2; used[dir]; i++ {
		dir = fmt.Sprintf("%s-%d", safeName(bn.Node), i)
	}
	used[dir] = true

	now := time.Now()
	for part := 1; ; part++ {
		n, err := io.ReadFull(resp.Body, buf)
		if n > 0 {
			hdr := &tar.Header{Name: fmt.Sprintf("nodes/%s/%06d.badger", dir, part), Mode: 0o644, Size: int64(n), ModTime: now}
			if werr := tw.WriteHeader(hdr); werr != nil {
				bn.Error = werr.Error()
				return bn
			}
			if _, werr := tw.Write(buf[:n]); werr != nil {
				bn.Error = werr.Error()
				return bn
			}
			bn.Bytes += int64(n)
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			bn.Error = fmt.Sprintf("backup interrupted: %v", err)
			return bn
		}
	}
	// The agent reports the outcome in trailers, after the stream.
	if e := resp.Trailer.Get("X-Kapture-Error"); e != "" {
		bn.Error = e
	}
	bn.Entries, _ = strconv.ParseInt(resp.Trailer.Get("X-Kapture-Entries"), 10, 64)
	return bn
}

// Restore reads an archive from r and sends each node's parts, in order, to
// an agent: the one of the same node, else the first reachable one.
func (c *Client) Restore(ctx context.Context, r io.Reader) ([]RestoreResult, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, errors.New("not a Kapture backup (expected a .tar.gz file)")
	}
	tr := tar.NewReader(gz)

	var agents []AgentStatus
	for _, a := range c.Agents() {
		if a.Status == "ok" {
			agents = append(agents, a)
		}
	}
	if len(agents) == 0 {
		return nil, errors.New("no reachable agent to restore into")
	}

	var out []RestoreResult
	var cur *restoreStream
	finish := func() {
		if cur != nil {
			out = append(out, cur.close())
			cur = nil
		}
	}
	valid := false
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			finish()
			return out, fmt.Errorf("the backup file is damaged or incomplete: %w", err)
		}
		if hdr.Name == manifestEntry {
			valid = true
			continue
		}
		node, ok := nodeOf(hdr.Name)
		if !ok {
			continue
		}
		valid = true
		if cur == nil || cur.node != node {
			finish()
			cur = openRestore(ctx, node, pickTarget(node, agents))
		}
		if _, err := io.Copy(cur.pw, tr); err != nil {
			cur.pw.CloseWithError(err)
		}
	}
	finish()
	if !valid {
		return nil, errors.New("not a Kapture backup (no manifest or node data)")
	}
	return out, nil
}

type restoreStream struct {
	node, target string
	pw           *io.PipeWriter
	done         chan RestoreResult
}

func openRestore(ctx context.Context, node string, target AgentStatus) *restoreStream {
	pr, pw := io.Pipe()
	s := &restoreStream{node: node, target: target.Node, pw: pw, done: make(chan RestoreResult, 1)}
	go func() {
		res := RestoreResult{Node: node, Target: target.Node}
		defer func() { s.done <- res }()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.Endpoint+"/api/v1/restore", pr)
		if err != nil {
			res.Error = err.Error()
			pr.CloseWithError(err)
			return
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		resp, err := streamClient.Do(req)
		if err != nil {
			res.Error = fmt.Sprintf("agent unreachable: %v", err)
			pr.CloseWithError(err)
			return
		}
		defer resp.Body.Close()
		var body struct {
			Restored int64  `json:"restored"`
			Error    string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body)
		res.Entries, res.Error = body.Restored, body.Error
		if res.Error == "" && resp.StatusCode != http.StatusOK {
			res.Error = fmt.Sprintf("agent replied HTTP %d", resp.StatusCode)
		}
		// Unblock the archive reader if the agent stopped reading early.
		pr.CloseWithError(errors.New("agent closed the restore"))
	}()
	return s
}

func (s *restoreStream) close() RestoreResult {
	s.pw.Close()
	return <-s.done
}

func pickTarget(node string, agents []AgentStatus) AgentStatus {
	for _, a := range agents {
		if safeName(a.Node) == node {
			return a
		}
	}
	return agents[0]
}

// nodeOf returns <node> of "nodes/<node>/<part>".
func nodeOf(name string) (string, bool) {
	rest, ok := strings.CutPrefix(name, "nodes/")
	if !ok {
		return "", false
	}
	node, part, ok := strings.Cut(rest, "/")
	return node, ok && node != "" && part != ""
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeName(s string) string {
	if s = unsafeChars.ReplaceAllString(s, "_"); s == "" {
		return "node"
	}
	return s
}

func hostOf(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return endpoint
}
