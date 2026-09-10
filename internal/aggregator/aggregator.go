package aggregator

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ordinary/k8s-log-catcher/internal/aggregator/discovery"
	"github.com/ordinary/k8s-log-catcher/internal/aggregator/fanout"
	"github.com/ordinary/k8s-log-catcher/internal/aggregator/handler"
	"github.com/ordinary/k8s-log-catcher/internal/auth"
	"github.com/ordinary/k8s-log-catcher/internal/config"
	"github.com/ordinary/k8s-log-catcher/web"
)

// Aggregator runs the query router and dashboard.
type Aggregator struct {
	cfg *config.Config
	srv *http.Server
}

// New creates a new Aggregator.
func New(cfg *config.Config) *Aggregator {
	return &Aggregator{cfg: cfg}
}

// Run starts the aggregator — blocks until Stop is called.
func (a *Aggregator) Run() error {
	// Setup discovery
	var disc discovery.Provider
	if a.cfg.Aggregator.Discovery.Method == "kubernetes" {
		disc = discovery.NewKubernetes(discovery.K8sOptions{
			Namespace:       a.cfg.Aggregator.Discovery.Namespace,
			LabelSelector:   a.cfg.Aggregator.Discovery.LabelSelector,
			HeadlessService: "kapture-agents",
			AgentPort:       a.cfg.Agent.API.Port,
			StaticEndpoints: a.cfg.Aggregator.Discovery.Endpoints,
		})
	} else {
		disc = discovery.NewStatic(a.cfg.Aggregator.Discovery.Endpoints)
	}

	// Setup fanout client
	fc := fanout.New(disc, a.cfg.Aggregator.Query.Timeout)

	// Setup Auth Manager
	authFile := a.cfg.Aggregator.Dashboard.Auth.FilePath
	if authFile == "" {
		authFile = "/data/kapture/auth.json"
	}
	authMgr, err := auth.NewManager(authFile, a.cfg.Aggregator.Dashboard.Auth.Enabled)
	if err != nil {
		return fmt.Errorf("init auth manager: %w", err)
	}

	// Setup handlers
	h := handler.New(fc, authMgr)

	mux := http.NewServeMux()

	// Get embedded static file server
	staticFS := web.StaticHandler()

	// Register routes
	h.RegisterRoutes(mux, staticFS)

	// Wrap with 2FA session auth middleware
	var finalHandler http.Handler = mux
	if a.cfg.Aggregator.Dashboard.Auth.Enabled {
		finalHandler = authMiddleware(mux, authMgr)
	}

	// Add CORS middleware
	finalHandler = corsMiddleware(finalHandler)

	port := a.cfg.Aggregator.Dashboard.Port
	a.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: finalHandler,
	}

	slog.Info("aggregator starting", "port", port,
		"agents", a.cfg.Aggregator.Discovery.Endpoints)

	return a.srv.ListenAndServe()
}

// Stop gracefully shuts down.
func (a *Aggregator) Stop() error {
	if a.srv != nil {
		return a.srv.Close()
	}
	return nil
}

func authMiddleware(next http.Handler, mgr *auth.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !mgr.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		// Allow static assets (HTML, CSS, JS, icons) freely so the in-app setup & login modal can render
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		// Allow public auth endpoints (status, setup, login)
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Check session token
		token := extractToken(r)
		if valid, _ := mgr.ValidateToken(token); valid {
			next.ServeHTTP(w, r)
			return
		}

		// Unauthorized
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":         "Unauthorized. Login with 2FA required.",
			"auth_required": true,
		})
	})
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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
