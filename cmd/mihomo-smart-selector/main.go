package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

var version = "dev"

func main() {
	configFlag := flag.String("config", "", "configuration path (Windows: config.yaml beside the executable; other systems: ./config.yaml)")
	initOnly := flag.Bool("init", false, "create the configuration if missing, then exit")
	showVersion := flag.Bool("version", false, "print version and platform, then exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("Mihomo Smart Selector %s (%s/%s; %s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	if err := start(*configFlag, *initOnly); err != nil {
		log.Printf("startup/service error: %v", err)
		pauseOnLaunchError()
		os.Exit(1)
	}
}

func start(explicit string, initOnly bool) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path, err := config.DefaultPath(explicit, executable, cwd, runtime.GOOS)
	if err != nil {
		return err
	}
	if explicit == "" || initOnly {
		created, err := config.Initialize(path)
		if err != nil {
			return err
		}
		if created {
			log.Printf("Created first-run configuration: %s", path)
		}
	}
	log.Printf("Configuration: %s", path)
	if initOnly {
		_, err := config.Load(path)
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runService(ctx, path)
}
