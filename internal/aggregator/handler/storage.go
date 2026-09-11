package handler

import (
	"log/slog"
	"net/http"
	"time"
)

// Backup and restore of the stored logs of every agent.
func (h *Handler) registerStorage(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/storage/backup", h.backup)
	mux.HandleFunc("POST /api/v1/storage/restore", h.restore)
}

// backup streams a .tar.gz with the logs of every agent, optionally limited
// to the days between from and to (YYYY-MM-DD, inclusive).
func (h *Handler) backup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	for _, d := range []string{from, to} {
		if d != "" && !dateRe.MatchString(d) {
			writeError(w, http.StatusBadRequest, "dates must be YYYY-MM-DD")
			return
		}
	}
	if len(h.fanout.Endpoints()) == 0 {
		writeError(w, http.StatusBadGateway, "no agents discovered")
		return
	}
	name := "kapture-backup-" + time.Now().In(h.loc).Format("20060102-150405") + ".tar.gz"
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	man, err := h.fanout.Backup(r.Context(), w, from, to, h.loc.String())
	if err != nil {
		slog.Error("backup failed", "error", err)
		return
	}
	for _, n := range man.Nodes {
		if n.Error != "" {
			slog.Warn("backup of an agent failed", "node", n.Node, "endpoint", n.Endpoint, "error", n.Error)
		}
	}
	slog.Info("backup written", "user", IdentityOf(r).Username, "nodes", len(man.Nodes), "from", from, "to", to)
}

// restore loads a backup made by /storage/backup. Each node's logs go to the
// agent of the same node, or to the first agent when that node is not here
// (restoring a cluster's backup on a laptop, for example).
func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	results, err := h.fanout.Restore(r.Context(), r.Body)
	var total int64
	for _, res := range results {
		total += res.Entries
	}
	out := map[string]any{"results": results, "restored": total}
	if err != nil {
		out["error"] = err.Error()
		writeJSON(w, http.StatusBadRequest, out)
		return
	}
	slog.Info("backup restored", "user", IdentityOf(r).Username, "lines", total)
	writeJSON(w, http.StatusOK, out)
}
