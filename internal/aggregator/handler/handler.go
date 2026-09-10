package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/aggregator/fanout"
	"github.com/ordinary/k8s-log-catcher/internal/auth"
	"github.com/ordinary/k8s-log-catcher/internal/model"
)

// Handler serves the aggregator REST API and dashboard.
type Handler struct {
	fanout  *fanout.Client
	authMgr *auth.Manager
}

// New creates a new handler.
func New(fc *fanout.Client, authMgr *auth.Manager) *Handler {
	return &Handler{fanout: fc, authMgr: authMgr}
}

// RegisterRoutes registers all API and dashboard routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, staticFS http.Handler) {
	// Auth & 2FA routes
	mux.HandleFunc("/api/v1/auth/status", h.authStatus)
	mux.HandleFunc("/api/v1/auth/setup/init", h.setupInit)
	mux.HandleFunc("/api/v1/auth/setup/complete", h.setupComplete)
	mux.HandleFunc("/api/v1/auth/login/credentials", h.loginCredentials)
	mux.HandleFunc("/api/v1/auth/login/2fa", h.login2FA)
	mux.HandleFunc("/api/v1/auth/login", h.login)
	mux.HandleFunc("/api/v1/auth/logout", h.logout)

	// Data & Query routes
	mux.HandleFunc("/api/v1/logs", h.queryLogs)
	mux.HandleFunc("DELETE /api/v1/logs", h.deleteLogs)
	mux.HandleFunc("DELETE /api/v1/logs/all", h.resetAll)
	mux.HandleFunc("/api/v1/dates", h.listDates)
	mux.HandleFunc("/api/v1/namespaces", h.listNamespaces)
	mux.HandleFunc("/api/v1/workloads", h.listWorkloads)
	mux.HandleFunc("/api/v1/pods", h.listPods)
	mux.HandleFunc("/api/v1/storage", h.storageInfo)
	mux.HandleFunc("/api/v1/health", h.health)

	// Dashboard static files
	mux.Handle("/", staticFS)
}

func (h *Handler) queryLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := model.QueryRequest{
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

	result, err := h.fanout.QueryLogs(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) deleteLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := model.DeleteRequest{
		BeforeDate: q.Get("before"),
		Namespace:  q.Get("namespace"),
		Workload:   q.Get("workload"),
	}
	deleted, err := h.fanout.DeleteLogs(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": deleted})
}

func (h *Handler) resetAll(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.fanout.DeleteLogs(model.DeleteRequest{All: true})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": deleted, "status": "all logs deleted"})
}

func (h *Handler) listDates(w http.ResponseWriter, r *http.Request) {
	dates, err := h.fanout.GetDates()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, dates)
}

func (h *Handler) listNamespaces(w http.ResponseWriter, r *http.Request) {
	ns, err := h.fanout.GetNamespaces()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ns)
}

func (h *Handler) listWorkloads(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	wl, err := h.fanout.GetWorkloads(namespace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wl)
}

func (h *Handler) listPods(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pods, err := h.fanout.GetPods(q.Get("namespace"), q.Get("workload"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pods)
}

func (h *Handler) storageInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.fanout.GetStorageInfo()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *Handler) authStatus(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"auth_enabled":  false,
			"setup_needed":  false,
			"authenticated": true,
			"username":      "anonymous",
		})
		return
	}

	setupNeeded := !h.authMgr.IsSetupCompleted()
	token := extractToken(r)
	authed, user := h.authMgr.ValidateToken(token)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"auth_enabled":  true,
		"setup_needed":  setupNeeded,
		"authenticated": authed,
		"username":      user,
	})
}

func (h *Handler) setupInit(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth is disabled"})
		return
	}
	if h.authMgr.IsSetupCompleted() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "setup has already been completed"})
		return
	}

	secret := auth.GenerateSecret()
	otpauthURL := auth.GenerateOTPAuthURL("admin", secret)
	pngBytes, err := auth.GenerateQRCodePNG(otpauthURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate QR code"})
		return
	}

	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"secret":      secret,
		"otpauth_url": otpauthURL,
		"qr_data_url": dataURL,
	})
}

func (h *Handler) setupComplete(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth is disabled"})
		return
	}
	if h.authMgr.IsSetupCompleted() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "setup has already been completed"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Secret   string `json:"secret"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	token, err := h.authMgr.CompleteSetup(req.Username, req.Password, req.Secret, req.Code)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"token":    token,
		"username": req.Username,
	})
}

// loginCredentials (Langkah 1): Verifikasi Username & Password terlebih dahulu.
func (h *Handler) loginCredentials(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"require_2fa": false,
			"token":       "anonymous-token",
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format data tidak valid"})
		return
	}

	tempToken, err := h.authMgr.ValidateCredentials(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"require_2fa": true,
		"temp_token":  tempToken,
		"username":    req.Username,
	})
}

// login2FA (Langkah 2): Setelah user/password valid, verifikasi 6-digit kode 2FA Google Authenticator.
func (h *Handler) login2FA(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"token":   "anonymous-token",
		})
		return
	}

	var req struct {
		TempToken string `json:"temp_token"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format data tidak valid"})
		return
	}

	token, err := h.authMgr.Verify2FALogin(req.TempToken, req.Code)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"token":   token,
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"token":   "anonymous-token",
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// If code is empty, perform Step 1 (credentials check only)
	if strings.TrimSpace(req.Code) == "" {
		tempToken, err := h.authMgr.ValidateCredentials(req.Username, req.Password)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"require_2fa": true,
			"temp_token":  tempToken,
			"username":    req.Username,
		})
		return
	}

	token, err := h.authMgr.Login(req.Username, req.Password, req.Code)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"token":    token,
		"username": req.Username,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" {
		h.authMgr.Logout(token)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if tok := r.Header.Get("X-Session-Token"); tok != "" {
		return tok
	}
	if c, err := r.Cookie("kapture_token"); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	status := h.fanout.HealthCheck()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"agents": status,
	})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
