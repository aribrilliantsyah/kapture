package agent

import (
	"log/slog"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/agent/enricher"
	"github.com/ordinary/k8s-log-catcher/internal/agent/hub"
	"github.com/ordinary/k8s-log-catcher/internal/agent/server"
	"github.com/ordinary/k8s-log-catcher/internal/agent/tailer"
	"github.com/ordinary/k8s-log-catcher/internal/config"
	"github.com/ordinary/k8s-log-catcher/internal/kube"
	"github.com/ordinary/k8s-log-catcher/internal/model"
	"github.com/ordinary/k8s-log-catcher/internal/storage"
	badgerstore "github.com/ordinary/k8s-log-catcher/internal/storage/badger"
)

// Agent runs the log collection pipeline on a single node.
type Agent struct {
	cfg        *config.Config
	store      storage.Store
	tailer     *tailer.Tailer
	hub        *hub.Hub
	http       *server.HTTPServer
	done       chan struct{}
	writerDone chan struct{}
}

// New creates a new Agent.
func New(cfg *config.Config) (*Agent, error) {
	store, err := badgerstore.New(badgerstore.Options{
		Path:        cfg.Agent.Storage.Path,
		Node:        cfg.NodeName,
		Location:    cfg.Loc,
		Retention:   cfg.Agent.Storage.Retention,
		MaxDisk:     cfg.Agent.Storage.MaxDisk,
		GCInterval:  cfg.Agent.Storage.GCInterval,
		Compression: cfg.Agent.Storage.Compression,
	})
	if err != nil {
		return nil, err
	}

	// Owner lookups group every generation of a deployment's pods together.
	kc, err := kube.InCluster()
	if err != nil {
		slog.Info("not running in a cluster, workloads are derived from pod names", "reason", err)
		kc = nil
	}
	h := hub.New()
	return &Agent{
		cfg:        cfg,
		store:      store,
		tailer:     tailer.New(cfg.Agent, cfg.NodeName, store, enricher.NewResolver(kc)),
		hub:        h,
		http:       server.NewHTTPServer(store, h, cfg.Agent.API.Port, cfg.NodeName),
		done:       make(chan struct{}),
		writerDone: make(chan struct{}),
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

	go a.batchWriter()
	if err := a.tailer.Start(); err != nil {
		return err
	}
	return a.http.Start()
}

// Stop drains the pipeline in order: files, pending batch, then storage.
func (a *Agent) Stop() {
	slog.Info("stopping agent")
	a.tailer.Stop()
	close(a.done)
	<-a.writerDone
	a.http.Stop()
	a.store.Close()
}

// batchWriter collects entries from the tailer and writes them in batches.
func (a *Agent) batchWriter() {
	defer close(a.writerDone)
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
		} else {
			a.hub.Publish(batch)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-a.done:
			for {
				select {
				case e := <-a.tailer.Out():
					batch = append(batch, e)
				default:
					flush()
					return
				}
			}
		case e := <-a.tailer.Out():
			batch = append(batch, e)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
