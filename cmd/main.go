package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fredericrous/cluster-vision/internal/server"
)

func main() {
	var cfg server.Config
	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.Kubeconfig, "kubeconfig", "", "path to kubeconfig (empty for in-cluster)")
	flag.StringVar(&cfg.TFStatePath, "tfstate", "", "path to terraform.tfstate file")
	flag.DurationVar(&cfg.RefreshInterval, "refresh", 5*time.Minute, "data refresh interval")
	flag.Parse()

	// Allow env var overrides
	if v := os.Getenv("KUBECONFIG"); v != "" && cfg.Kubeconfig == "" {
		cfg.Kubeconfig = v
	}
	if v := os.Getenv("TFSTATE_PATH"); v != "" && cfg.TFStatePath == "" {
		cfg.TFStatePath = v
	}

	slog.Info("cluster-vision starting",
		"port", cfg.Port,
		"kubeconfig", cfg.Kubeconfig,
		"tfstate", cfg.TFStatePath,
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
