package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/api"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/connection"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

// Rebuild the runtime in the same process. This works with go run, portable
// Windows executables and service supervisors, without spawning orphan workers.
func runService(ctx context.Context, path string) (result error) {
	storage := &runtimeStorage{}
	defer func() {
		if storage.store != nil {
			result = errors.Join(result, storage.store.Close())
		}
	}()
	for ctx.Err() == nil {
		restart, err := serveOnce(ctx, path, storage)
		if err != nil || !restart {
			return err
		}
		log.Print("Restarting Mihomo Smart Selector with saved configuration...")
	}
	return nil
}

// Storage belongs to the process, while requests and workers belong to one
// generation. Keeping the same pool avoids reopening/migrating a Windows WAL
// database while database/sql finishes a cancelled transaction's rollback.
type runtimeStorage struct {
	store *history.Store
	path  string
}

func serveOnce(ctx context.Context, path string, storage *runtimeStorage) (bool, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return false, err
	}
	originalController := cfg.Mihomo.Controller
	connections, effectiveMihomo, controller, err := connection.Open(cfg.Mihomo, cfg.Storage.Path+".connection.json")
	if err != nil {
		return false, err
	}
	// Claim the address before touching a database or starting background jobs.
	listener, err := net.Listen("tcp", cfg.HTTP.Listen)
	if err != nil {
		return false, fmt.Errorf("cannot listen on %s; another instance may be running, or http.listen must be changed in %s: %w", cfg.HTTP.Listen, path, err)
	}
	defer listener.Close()
	cfg.Mihomo = effectiveMihomo
	if storage.store == nil {
		storage.store, err = history.Open(cfg.Storage.Path)
		if err != nil {
			return false, err
		}
		storage.path = cfg.Storage.Path
	} else if storage.path != cfg.Storage.Path {
		return false, fmt.Errorf("storage path changed during restart; restart from the launch terminal")
	}
	store := storage.store
	if err := store.BindLegacyController(ctx, originalController); err != nil {
		return false, err
	}
	if err := store.RecoverInterrupted(ctx); err != nil {
		return false, err
	}
	manager := scan.NewManager(cfg, controller, store)
	if err := manager.LoadSettings(ctx); err != nil {
		return false, err
	}
	monitoring, err := monitor.New(store, manager)
	if err != nil {
		return false, err
	}
	apiServer, err := api.New(cfg.HTTP, manager, controller)
	if err != nil {
		return false, err
	}
	apiServer.WithMonitor(monitoring)
	apiServer.WithConnection(connections)
	var boot [16]byte
	if _, err := rand.Read(boot[:]); err != nil {
		return false, err
	}
	restarts := make(chan struct{}, 1)
	apiServer.WithServiceControl(hex.EncodeToString(boot[:]), version, func() error {
		// Validate before draining a working instance. Network reachability is
		// intentionally not required: reconnecting is a reason to restart.
		next, err := config.Load(path)
		if err != nil {
			return fmt.Errorf("无法读取有效配置，重启未执行；请检查配置文件后重试")
		}
		if !reflect.DeepEqual(next.HTTP, cfg.HTTP) || next.Storage.Path != cfg.Storage.Path {
			return fmt.Errorf("监听地址、访问权限或存储位置已改变，请在启动终端中重启服务")
		}
		if _, _, _, err := connection.Open(next.Mihomo, next.Storage.Path+".connection.json"); err != nil {
			return fmt.Errorf("无法读取已保存的连接或密钥，重启未执行；请修正连接配置后重试")
		}
		return manager.PrepareRestart()
	}, func() { restarts <- struct{}{} })
	requests, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	server := &http.Server{
		Handler: apiServer.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		BaseContext: func(net.Listener) context.Context { return requests },
	}
	manager.StartMaintenance()
	monitoring.Start()
	serveError := make(chan error, 1)
	go func() { serveError <- server.Serve(listener) }()
	log.Printf("Mihomo Smart Selector listening on %s", listener.Addr())
	log.Printf("Open http://%s in your browser. Press Ctrl+C to stop.", listener.Addr())
	restart := false
	var runtimeError error
	select {
	case <-ctx.Done():
	case <-restarts:
		restart = true
	case err := <-serveError:
		if !errors.Is(err, http.ErrServerClosed) {
			runtimeError = err
		}
	}
	cancelRequests() // End SSE streams and bound all in-flight HTTP work.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	httpStopped := make(chan error, 1)
	go func() { httpStopped <- server.Shutdown(shutdownCtx) }()
	monitorError := monitoring.Shutdown(shutdownCtx)
	scanError := manager.Shutdown(shutdownCtx)
	httpError := <-httpStopped
	if httpError != nil {
		_ = server.Close()
	}
	if err := errors.Join(runtimeError, monitorError, scanError, httpError); err != nil {
		// Never start another runtime alongside workers that did not drain.
		return false, fmt.Errorf("service shutdown did not finish: %w", err)
	}
	return restart && ctx.Err() == nil, nil
}
