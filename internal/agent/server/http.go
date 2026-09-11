package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/agent/hub"
	"github.com/ordinary/k8s-log-catcher/internal/logfilter"
	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/storage"
	"github.com/ordinary/k8s-log-catcher/internal/version"
)

// HTTPServer serves health endpoints and the local query API of an agent.
type HTTPServer struct {
	store storage.Store
	hub   *hub.Hub
	port  int
	node  string
	srv   *http.Server
}

// NewHTTPServer creates a new agent HTTP server.
func NewHTTPServer(store storage.Store, h *hub.Hub, port int, node string) *HTTPServer {
	s := &HTTPServer{store: store, hub: h, port: port, node: node}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.healthz)
	mux.HandleFunc("GET /api/v1/logs", s.queryLogs)
	mux.HandleFunc("GET /api/v1/stats/volume", s.volume)
	mux.HandleFunc("GET /api/v1/stats/recap", s.recap)
	mux.HandleFunc("GET /api/v1/tail", s.tail)
	mux.HandleFunc("GET /api/v1/catalog", s.catalog)
	mux.HandleFunc("GET /api/v1/dates", s.listDates)
	mux.HandleFunc("GET /api/v1/namespaces", s.listNamespaces)
	mux.HandleFunc("GET /api/v1/workloads", s.listWorkloads)
	mux.HandleFunc("GET /api/v1/pods", s.listPods)
	mux.HandleFunc("GET /api/v1/storage", s.storageInfo)
	mux.HandleFunc("DELETE /api/v1/logs", s.deleteLogs)
	mux.HandleFunc("DELETE /api/v1/logs/all", s.resetAll)
	mux.HandleFunc("GET /api/v1/backup", s.backup)
	mux.HandleFunc("POST /api/v1/restore", s.restore)

	s.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// Start begins listening.
func (s *HTTPServer) Start() error {
	slog.Info("agent HTTP server starting", "port", s.port)
	if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop shuts the server down.
func (s *HTTPServer) Stop() error {
	return s.srv.Close()
}

func (s *HTTPServer) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "node": s.node, "version": version.Version})
}

func (s *HTTPServer) queryLogs(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.Query(model.ParseQuery(r.URL.Query()))
	reply(w, result, err)
}

func (s *HTTPServer) volume(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.Volume(model.ParseQuery(r.URL.Query()))
	reply(w, result, err)
}

func (s *HTTPServer) recap(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	writeJSON(w, http.StatusOK, s.store.Recap(q.Get("from"), q.Get("to"), q.Get("namespace")))
}

// tail streams entries written from now on as newline-delimited JSON. A blank
// line every 15s keeps idle connections open through proxies.
func (s *HTTPServer) tail(w http.ResponseWriter, r *http.Request) {
	f, err := logfilter.New(model.ParseQuery(r.URL.Query()))
	if err != nil {
		reply(w, nil, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	sub := s.hub.Subscribe(f)
	defer s.hub.Unsubscribe(sub)

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	enc := json.NewEncoder(w)
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-sub.C:
			if enc.Encode(e) != nil {
				return
			}
			for n := 0; n < 500 && len(sub.C) > 0; n++ { // send what is queued in one flush
				if enc.Encode(<-sub.C) != nil {
					return
				}
			}
			flusher.Flush()
		case <-ping.C:
			if _, err := w.Write([]byte("\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *HTTPServer) catalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Catalog())
}

func (s *HTTPServer) listDates(w http.ResponseWriter, _ *http.Request) {
	dates, err := s.store.Dates()
	reply(w, dates, err)
}

func (s *HTTPServer) listNamespaces(w http.ResponseWriter, _ *http.Request) {
	ns, err := s.store.Namespaces()
	reply(w, ns, err)
}

func (s *HTTPServer) listWorkloads(w http.ResponseWriter, r *http.Request) {
	wl, err := s.store.Workloads(r.URL.Query().Get("namespace"))
	reply(w, wl, err)
}

func (s *HTTPServer) listPods(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pods, err := s.store.Pods(q.Get("namespace"), q.Get("workload"))
	reply(w, pods, err)
}

func (s *HTTPServer) storageInfo(w http.ResponseWriter, _ *http.Request) {
	info, err := s.store.StorageInfo()
	reply(w, info, err)
}

func (s *HTTPServer) deleteLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	deleted, err := s.store.Delete(model.DeleteRequest{
		Date:       q.Get("date"),
		BeforeDate: q.Get("before"),
		Namespace:  q.Get("namespace"),
		Workload:   q.Get("workload"),
	})
	reply(w, map[string]int64{"deleted": deleted}, err)
}

func (s *HTTPServer) resetAll(w http.ResponseWriter, _ *http.Request) {
	deleted, err := s.store.Delete(model.DeleteRequest{All: true})
	reply(w, map[string]int64{"deleted": deleted}, err)
}

// backup streams this node's logs. The line count and any failure come as
// trailers, since the body is already on its way when they are known.
func (s *HTTPServer) backup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Kapture-Node", s.node)
	w.Header().Set("Trailer", "X-Kapture-Entries, X-Kapture-Error")
	n, err := s.store.Backup(w, q.Get("from"), q.Get("to"))
	w.Header().Set("X-Kapture-Entries", strconv.FormatInt(n, 10))
	if err != nil {
		slog.Error("backup failed", "error", err)
		w.Header().Set("X-Kapture-Error", err.Error())
		return
	}
	slog.Info("backup sent", "lines", n)
}

func (s *HTTPServer) restore(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.Restore(r.Body)
	if err != nil {
		slog.Error("restore failed", "restored", n, "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{"restored": n, "error": err.Error()})
		return
	}
	slog.Info("backup restored", "lines", n)
	writeJSON(w, http.StatusOK, map[string]any{"restored": n, "node": s.node})
}

// reply writes v, or err as a 400 (bad filters are the only expected failure).
func reply(w http.ResponseWriter, v any, err error) {
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
