package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/server"
)

func main() {
	var cfg server.Config
	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.Kubeconfig, "kubeconfig", "", "path to kubeconfig (empty for in-cluster)")
	flag.DurationVar(&cfg.RefreshInterval, "refresh", 5*time.Minute, "data refresh interval")
	flag.Parse()

	// Allow env var overrides
	if v := os.Getenv("KUBECONFIG"); v != "" && cfg.Kubeconfig == "" {
		cfg.Kubeconfig = v
	}
	if v := os.Getenv("CLUSTER_NAME"); v != "" {
		cfg.ClusterName = v
	}

	// Data sources from env
	if v := os.Getenv("DATA_SOURCES"); v != "" {
		var sources []model.DataSource
		if err := json.Unmarshal([]byte(v), &sources); err != nil {
			slog.Error("failed to parse DATA_SOURCES", "error", err)
			os.Exit(1)
		}
		cfg.DataSources = sources
	}

	// Backward compat: TFSTATE_PATH creates a single tfstate source
	if v := os.Getenv("TFSTATE_PATH"); v != "" && len(cfg.DataSources) == 0 {
		cfg.DataSources = []model.DataSource{{
			Name: "Terraform",
			Type: "tfstate",
			Path: v,
		}}
	}

	slog.Info("cluster-vision starting",
		"port", cfg.Port,
		"kubeconfig", cfg.Kubeconfig,
		"dataSources", len(cfg.DataSources),
		"refresh", cfg.RefreshInterval,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv, err := server.New(cfg)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
