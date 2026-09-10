package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ordinary/k8s-log-catcher/internal/agent"
	"github.com/ordinary/k8s-log-catcher/internal/aggregator"
	"github.com/ordinary/k8s-log-catcher/internal/config"
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

	slog.Info("k8s-log-catcher starting", "mode", cfg.Mode, "version", "1.0.0")

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
