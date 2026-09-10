package agent

import (
	"log/slog"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/agent/server"
	"github.com/ordinary/k8s-log-catcher/internal/agent/tailer"
	"github.com/ordinary/k8s-log-catcher/internal/config"
	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/storage"
	badgerstore "github.com/ordinary/k8s-log-catcher/internal/storage/badger"
)

// Agent runs the log collection pipeline on a single node.
type Agent struct {
	cfg    *config.Config
	store  storage.Store
	tailer *tailer.Tailer
	http   *server.HTTPServer
	done   chan struct{}
}

// New creates a new Agent.
func New(cfg *config.Config) (*Agent, error) {
	// Open storage
	store, err := badgerstore.New(badgerstore.Options{
		Path:        cfg.Agent.Storage.Path,
		Retention:   cfg.Agent.Storage.Retention,
		MaxDisk:     cfg.Agent.Storage.MaxDisk,
		GCInterval:  cfg.Agent.Storage.GCInterval,
		Compression: cfg.Agent.Storage.Compression,
	})
	if err != nil {
		return nil, err
	}

	t := tailer.New(cfg.Agent, cfg.NodeName, store)
	httpSrv := server.NewHTTPServer(store, cfg.Agent.API.Port)

	return &Agent{
		cfg:    cfg,
		store:  store,
		tailer: t,
		http:   httpSrv,
		done:   make(chan struct{}),
	}, nil
}

// Run starts the agent — blocks until Stop is called.
func (a *Agent) Run() error {
	slog.Info("starting agent",
		"node", a.cfg.NodeName,
		"log_path", a.cfg.Agent.LogPath,
		"storage_path", a.cfg.Agent.Storage.Path,
		"retention", a.cfg.Agent.Storage.Retention,
	)

	// Start the batch writer goroutine
	go a.batchWriter()

	// Start tailing log files
	if err := a.tailer.Start(); err != nil {
		return err
	}

	// Start HTTP server (blocks)
	return a.http.Start()
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop() {
	slog.Info("stopping agent")
	close(a.done)
	a.tailer.Stop()
	a.http.Stop()
	a.store.Close()
}

// batchWriter collects log entries from the tailer channel and writes them
// to storage in batches for efficiency.
func (a *Agent) batchWriter() {
	batchSize := a.cfg.Agent.Collector.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	interval := a.cfg.Agent.Collector.BatchInterval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}

	batch := make([]model.LogEntry, 0, batchSize)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := a.store.Write(batch); err != nil {
			slog.Error("failed to write batch", "error", err, "count", len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-a.done:
			flush()
			return

		case entry := <-a.tailer.Out():
			batch = append(batch, entry)
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}
