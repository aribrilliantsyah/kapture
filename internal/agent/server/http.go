package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/storage"
)

// HTTPServer serves health endpoints and a local query API for the agent.
type HTTPServer struct {
	store storage.Store
	port  int
	srv   *http.Server
}

// NewHTTPServer creates a new agent HTTP server.
func NewHTTPServer(store storage.Store, port int) *HTTPServer {
	s := &HTTPServer{
		store: store,
		port:  port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)
	mux.HandleFunc("/api/v1/logs", s.queryLogs)
	mux.HandleFunc("/api/v1/dates", s.listDates)
	mux.HandleFunc("/api/v1/namespaces", s.listNamespaces)
	mux.HandleFunc("/api/v1/workloads", s.listWorkloads)
	mux.HandleFunc("/api/v1/pods", s.listPods)
	mux.HandleFunc("/api/v1/storage", s.storageInfo)
	mux.HandleFunc("DELETE /api/v1/logs", s.deleteLogs)
	mux.HandleFunc("DELETE /api/v1/logs/all", s.resetAll)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	return s
}

// Start begins listening.
func (s *HTTPServer) Start() error {
	slog.Info("agent HTTP server starting", "port", s.port)
	return s.srv.ListenAndServe()
}

// Stop gracefully shuts down.
func (s *HTTPServer) Stop() error {
	return s.srv.Close()
}

func (s *HTTPServer) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *HTTPServer) queryLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := model.QueryRequest{
		Date:      q.Get("date"),
		Namespace: q.Get("namespace"),
		Workload:  q.Get("workload"),
		WorkloadType: q.Get("workload_type"),
		Pod:       q.Get("pod"),
		Container: q.Get("container"),
		Level:     q.Get("level"),
		Search:    q.Get("search"),
		Regex:     q.Get("regex"),
		Cursor:    q.Get("cursor"),
		Sort:      q.Get("sort"),
	}

	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			req.Limit = n
		}
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			req.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			req.To = &t
		}
	}
	if v := q.Get("exclude_ns"); v != "" {
		req.ExcludeNS = splitComma(v)
	}

	result, err := s.store.Query(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *HTTPServer) listDates(w http.ResponseWriter, _ *http.Request) {
	dates, err := s.store.Dates()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, dates)
}

func (s *HTTPServer) listNamespaces(w http.ResponseWriter, _ *http.Request) {
	ns, err := s.store.Namespaces()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ns)
}

func (s *HTTPServer) listWorkloads(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	wl, err := s.store.Workloads(namespace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wl)
}

func (s *HTTPServer) listPods(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pods, err := s.store.Pods(q.Get("namespace"), q.Get("workload"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pods)
}

func (s *HTTPServer) storageInfo(w http.ResponseWriter, _ *http.Request) {
	info, err := s.store.StorageInfo()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *HTTPServer) deleteLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := model.DeleteRequest{
		BeforeDate: q.Get("before"),
		Namespace:  q.Get("namespace"),
		Workload:   q.Get("workload"),
	}

	deleted, err := s.store.Delete(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": deleted})
}

func (s *HTTPServer) resetAll(w http.ResponseWriter, _ *http.Request) {
	err := s.store.ResetAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "all logs deleted"})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func splitComma(s string) []string {
	var result []string
	for _, part := range splitStr(s, ",") {
		if p := trimStr(part); p != "" {
			result = append(result, p)
		}
	}
	return result
}

func splitStr(s, sep string) []string {
	result := []string{}
	for len(s) > 0 {
		idx := indexOf(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimStr(s string) string {
	start := 0
	for start < len(s) && s[start] == ' ' {
		start++
	}
	end := len(s)
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}
