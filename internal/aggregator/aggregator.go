package aggregator

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aribrilliantsyah/kapture/internal/aggregator/discovery"
	"github.com/aribrilliantsyah/kapture/internal/aggregator/fanout"
	"github.com/aribrilliantsyah/kapture/internal/aggregator/handler"
	"github.com/aribrilliantsyah/kapture/internal/auth"
	"github.com/aribrilliantsyah/kapture/internal/config"
	"github.com/aribrilliantsyah/kapture/web"
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

// Run starts the aggregator; it blocks until Stop is called.
func (a *Aggregator) Run() error {
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
	fc := fanout.New(disc, a.cfg.Aggregator.Query.Timeout)

	authMgr, err := auth.NewManager(a.cfg.Aggregator.Dashboard.Auth.FilePath, a.cfg.Aggregator.Dashboard.Auth.Enabled)
	if err != nil {
		return fmt.Errorf("init auth manager: %w", err)
	}

	h := handler.New(fc, authMgr, a.cfg.Aggregator.Query.MaxResults, a.cfg.Loc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, web.StaticHandler())

	port := a.cfg.Aggregator.Dashboard.Port
	a.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           h.Wrap(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("aggregator starting", "port", port, "discovery", a.cfg.Aggregator.Discovery.Method,
		"agents", disc.Endpoints(), "auth", authMgr.IsEnabled(), "timezone", a.cfg.Loc)

	if err := a.srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop shuts the server down.
func (a *Aggregator) Stop() error {
	if a.srv != nil {
		return a.srv.Close()
	}
	return nil
}
