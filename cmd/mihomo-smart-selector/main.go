package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/api"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/connection"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to selector YAML configuration")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	originalController := cfg.Mihomo.Controller
	connections, effectiveMihomo, controller, err := connection.Open(cfg.Mihomo, cfg.Storage.Path+".connection.json")
	if err != nil {
		log.Fatalf("controller configuration error: %v", err)
	}
	cfg.Mihomo = effectiveMihomo
	store, err := history.Open(cfg.Storage.Path)
	if err != nil {
		log.Fatalf("storage error: %v", err)
	}
	defer store.Close()
	if err := store.BindLegacyController(context.Background(), originalController); err != nil {
		log.Fatalf("controller history migration error: %v", err)
	}
	if err := store.RecoverInterrupted(context.Background()); err != nil {
		log.Fatalf("recovery error: %v", err)
	}

	manager := scan.NewManager(cfg, controller, store)
	if err := manager.LoadSettings(context.Background()); err != nil {
		log.Fatalf("runtime settings error: %v", err)
	}
	manager.StartMaintenance()
	monitoring, err := monitor.New(store, manager)
	if err != nil {
		log.Fatalf("monitor recovery error: %v", err)
	}
	apiServer, err := api.New(cfg.HTTP, manager, controller)
	if err != nil {
		log.Fatalf("HTTP server configuration error: %v", err)
	}
	apiServer.WithMonitor(monitoring)
	apiServer.WithConnection(connections)
	monitoring.Start()
	server := &http.Server{
		Addr:              cfg.HTTP.Listen,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Mihomo Smart Selector listening on %s", cfg.HTTP.Listen)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := monitoring.Shutdown(shutdownCtx); err != nil {
		log.Printf("monitor shutdown error: %v", err)
	}
	if err := manager.Shutdown(shutdownCtx); err != nil {
		log.Printf("scan shutdown error: %v", err)
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}
