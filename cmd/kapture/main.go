package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // zone database inside the binary, no OS tzdata needed

	"github.com/aribrilliantsyah/kapture/internal/agent"
	"github.com/aribrilliantsyah/kapture/internal/aggregator"
	"github.com/aribrilliantsyah/kapture/internal/config"
	"github.com/aribrilliantsyah/kapture/internal/version"
)

func main() {
	mode := flag.String("mode", "", "Run mode: agent or aggregator")
	flag.Parse()

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	if *mode != "" {
		cfg.Mode = *mode
	}

	// Setup logger
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	loc, err := cfg.Location()
	if err != nil {
		slog.Error("invalid KAPTURE_TIMEZONE, use an IANA name like Asia/Jakarta", "timezone", cfg.Timezone, "error", err)
		os.Exit(1)
	}
	cfg.Loc = loc

	slog.Info("kapture starting", "mode", cfg.Mode, "version", version.Version, "commit", version.Commit, "timezone", loc.String())

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	switch cfg.Mode {
	case "agent":
		runAgent(cfg, sigCh)
	case "aggregator":
		runAggregator(cfg, sigCh)
	default:
		slog.Error("unknown mode, use --mode=agent or --mode=aggregator", "mode", cfg.Mode)
		os.Exit(1)
	}
}

func runAgent(cfg *config.Config, sigCh chan os.Signal) {
	a, err := agent.New(cfg)
	if err != nil {
		slog.Error("failed to create agent", "error", err)
		os.Exit(1)
	}

	go func() {
		<-sigCh
		slog.Info("received shutdown signal")
		a.Stop()
	}()

	if err := a.Run(); err != nil {
		slog.Error("agent failed", "error", err)
		os.Exit(1)
	}
}

func runAggregator(cfg *config.Config, sigCh chan os.Signal) {
	a := aggregator.New(cfg)

	go func() {
		<-sigCh
		slog.Info("received shutdown signal")
		a.Stop()
	}()

	if err := a.Run(); err != nil {
		slog.Error("aggregator failed", "error", err)
		os.Exit(1)
	}
}
