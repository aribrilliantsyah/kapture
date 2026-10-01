package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/aribrilliantsyah/kapture/internal/aggregator/fanout"
	"github.com/aribrilliantsyah/kapture/internal/logfilter"
	"github.com/aribrilliantsyah/kapture/internal/model"
)

const (
	liveFlushEvery = 250 * time.Millisecond
	liveMaxPending = 2000
)

type liveMessage struct {
	Type    string           `json:"type"` // hello | entries
	Agents  int              `json:"agents,omitempty"`
	Entries []model.LogEntry `json:"entries,omitempty"`
}

// tail upgrades to a WebSocket and pushes new log lines matching the query
// (same parameters as /api/v1/logs). Lines are batched every 250ms, oldest
// first. The session cookie authenticates the upgrade; the library rejects
// cross-origin upgrades.
func (h *Handler) tail(w http.ResponseWriter, r *http.Request) {
	req := model.ParseQuery(r.URL.Query())
	if _, err := logfilter.New(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	// The client never sends data; CloseRead handles pings and the close frame.
	ctx := conn.CloseRead(r.Context())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	in := make(chan model.LogEntry, 4096)
	go h.fanout.Tail(ctx, req, in)

	send := func(m liveMessage) error {
		wctx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()
		return wsjson.Write(wctx, conn, m)
	}
	if send(liveMessage{Type: "hello", Agents: len(h.fanout.Agents())}) != nil {
		return
	}

	flush := time.NewTicker(liveFlushEvery)
	defer flush.Stop()
	var pending []model.LogEntry
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case e := <-in:
			pending = append(pending, e)
			if len(pending) > liveMaxPending { // a firehose: keep the newest lines
				pending = pending[len(pending)-liveMaxPending:]
			}
		case <-flush.C:
			if len(pending) == 0 {
				continue
			}
			fanout.SortAsc(pending)
			if send(liveMessage{Type: "entries", Entries: pending}) != nil {
				return
			}
			pending = nil
		}
	}
}

// recap returns daily per-workload rollups, by default for the last 14 days.
func (h *Handler) recap(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	for _, d := range []string{from, to} {
		if d != "" && !dateRe.MatchString(d) {
			writeError(w, http.StatusBadRequest, "dates must be YYYY-MM-DD")
			return
		}
	}
	if from == "" {
		from = time.Now().In(h.loc).AddDate(0, 0, -13).Format("2006-01-02")
	}
	days, errs := h.fanout.Recap(from, to, q.Get("namespace"))
	writeJSON(w, http.StatusOK, map[string]any{"from": from, "to": to, "days": days, "errors": errs})
}
