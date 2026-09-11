package handler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/aggregator/fanout"
	"github.com/ordinary/k8s-log-catcher/internal/auth"
	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/version"
	"github.com/ordinary/k8s-log-catcher/web"
)

// Handler serves the aggregator REST API and dashboard.
type Handler struct {
	fanout     *fanout.Client
	authMgr    *auth.Manager
	maxResults int
	loc        *time.Location
}

// New creates a new handler. maxResults caps exports; loc is the cluster time zone.
func New(fc *fanout.Client, authMgr *auth.Manager, maxResults int, loc *time.Location) *Handler {
	if loc == nil {
		loc = time.UTC
	}
	if maxResults <= 0 {
		maxResults = 10000
	}
	return &Handler{fanout: fc, authMgr: authMgr, maxResults: maxResults, loc: loc}
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// RegisterRoutes registers all API and dashboard routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, staticFS http.Handler) {
	// Public liveness probe (the /api/v1 routes require a session).
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	h.registerAuth(mux)
	h.registerUsers(mux)
	h.registerStorage(mux)
	mux.Handle("GET /login", web.Page("auth.html"))
	mux.Handle("GET /setup", web.Page("auth.html"))
	mux.Handle("GET /{$}", web.Page("index.html"))

	mux.HandleFunc("GET /api/v1/logs", h.queryLogs)
	mux.HandleFunc("GET /api/v1/logs/export", h.exportLogs)
	mux.HandleFunc("DELETE /api/v1/logs", h.deleteLogs)
	mux.HandleFunc("DELETE /api/v1/logs/all", h.resetAll)
	mux.HandleFunc("GET /api/v1/stats/volume", h.volume)
	mux.HandleFunc("GET /api/v1/stats/recap", h.recap)
	mux.HandleFunc("GET /api/v1/tail", h.tail)
	mux.HandleFunc("GET /api/v1/catalog", h.catalog)
	mux.HandleFunc("GET /api/v1/dates", h.listDates)
	mux.HandleFunc("GET /api/v1/namespaces", h.listNamespaces)
	mux.HandleFunc("GET /api/v1/workloads", h.listWorkloads)
	mux.HandleFunc("GET /api/v1/pods", h.listPods)
	mux.HandleFunc("GET /api/v1/nodes", h.nodes)
	mux.HandleFunc("GET /api/v1/storage", h.storageInfo)
	mux.HandleFunc("GET /api/v1/health", h.health)
	mux.HandleFunc("GET /api/v1/config", h.config)

	mux.Handle("/", staticFS)
}

func (h *Handler) queryLogs(w http.ResponseWriter, r *http.Request) {
	result, err := h.fanout.QueryLogs(model.ParseQuery(r.URL.Query()))
	reply(w, result, err)
}

func (h *Handler) volume(w http.ResponseWriter, r *http.Request) {
	result, err := h.fanout.Volume(model.ParseQuery(r.URL.Query()))
	reply(w, result, err)
}

func (h *Handler) catalog(w http.ResponseWriter, _ *http.Request) {
	items, errs := h.fanout.Catalog()
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "errors": errs})
}

// exportLogs streams up to maxResults matching entries as CSV or JSON.
func (h *Handler) exportLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := model.ParseQuery(q)
	max := h.maxResults
	if req.Limit > 0 && req.Limit < max {
		max = req.Limit
	}
	req.Limit = min(1000, max)

	// Fetch the first page before writing headers so errors stay JSON.
	page, err := h.fanout.QueryLogs(req)
	if err != nil {
		reply(w, nil, err)
		return
	}

	format := "json"
	if q.Get("format") == "csv" {
		format = "csv"
	}
	name := "kapture-logs-" + time.Now().UTC().Format("20060102-150405") + "." + format
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)

	var cw *csv.Writer
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		cw = csv.NewWriter(w)
		cw.Write([]string{"timestamp", "namespace", "workload", "workload_type", "pod", "container", "node", "stream", "level", "message"})
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[\n"))
	}

	written := 0
	for {
		for _, e := range page.Entries {
			if cw != nil {
				cw.Write([]string{e.Timestamp.Format(time.RFC3339Nano), e.Namespace, e.Workload, e.WorkloadType, e.Pod, e.Container, e.Node, e.Stream, e.Level, e.Message})
				continue
			}
			b, _ := json.Marshal(e)
			if written > 0 {
				w.Write([]byte(",\n"))
			}
			w.Write(b)
			written++
		}
		if cw != nil {
			written += len(page.Entries)
		}
		if page.NextCursor == "" || len(page.Entries) == 0 || written >= max {
			break
		}
		req.Cursor, req.Limit = page.NextCursor, min(1000, max-written)
		if page, err = h.fanout.QueryLogs(req); err != nil {
			break
		}
	}
	if cw != nil {
		cw.Flush()
	} else {
		w.Write([]byte("\n]\n"))
	}
}

func (h *Handler) deleteLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := model.DeleteRequest{
		Date:       q.Get("date"),
		BeforeDate: q.Get("before"),
		Namespace:  q.Get("namespace"),
		Workload:   q.Get("workload"),
	}
	for _, d := range []string{req.Date, req.BeforeDate} {
		if d != "" && !dateRe.MatchString(d) {
			writeError(w, http.StatusBadRequest, "dates must be YYYY-MM-DD")
			return
		}
	}
	if req == (model.DeleteRequest{}) {
		writeError(w, http.StatusBadRequest, "set date, before, namespace or workload (use DELETE /api/v1/logs/all to reset)")
		return
	}
	deleted, errs := h.fanout.DeleteLogs(req)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted, "errors": errs})
}

func (h *Handler) resetAll(w http.ResponseWriter, _ *http.Request) {
	deleted, errs := h.fanout.DeleteLogs(model.DeleteRequest{All: true})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted, "errors": errs})
}

func (h *Handler) listDates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.fanout.GetDates())
}

func (h *Handler) listNamespaces(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.fanout.GetNamespaces())
}

func (h *Handler) listWorkloads(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.fanout.GetWorkloads(r.URL.Query().Get("namespace")))
}

func (h *Handler) listPods(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	writeJSON(w, http.StatusOK, h.fanout.GetPods(q.Get("namespace"), q.Get("workload")))
}

func (h *Handler) nodes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.fanout.Agents())
}

func (h *Handler) storageInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.fanout.GetStorageInfo())
}

// config tells the dashboard how to render; it never calls agents, so the
// page can start even when an agent is slow.
func (h *Handler) config(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version.Version, "commit": version.Commit, "timezone": h.loc.String()})
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"timezone": h.loc.String(),
		"version":  version.Version,
		"commit":   version.Commit,
		"agents":   h.fanout.Agents(),
	})
}

// reply writes v, mapping agent rejections to their status code.
func reply(w http.ResponseWriter, v any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, v)
		return
	}
	code := http.StatusBadGateway
	var se *fanout.StatusError
	if errors.As(err, &se) {
		code = se.Code
	}
	writeError(w, code, err.Error())
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
