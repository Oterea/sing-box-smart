package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sing-box-smart/internal/app"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/demo"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/gateway"
	"sing-box-smart/internal/httpapi"
	"sing-box-smart/internal/real"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	cfg := config.Default()
	var pins string
	flag.StringVar(&cfg.Mode, "mode", cfg.Mode, "demo or real")
	flag.StringVar(&cfg.API, "api", cfg.API, "sing-box Clash API base URL")
	flag.StringVar(&cfg.Root, "root", cfg.Root, "root selector used for API health")
	flag.StringVar(&cfg.TestURL, "test-url", cfg.TestURL, "URL used for delay checks")
	flag.StringVar(&pins, "pins", "", "airport PINs: Name=selector,Name=selector")
	flag.StringVar(&cfg.Listen, "listen", cfg.Listen, "loopback HTTP listen address")
	flag.StringVar(&cfg.LogDir, "log-dir", cfg.LogDir, "log directory")
	flag.Parse()
	if pins != "" {
		parsed, err := config.ParsePins(pins)
		if err != nil {
			return err
		}
		cfg.Pins = parsed
	}
	if cfg.Mode == "real" && len(cfg.Pins) == 0 {
		return fmt.Errorf("real mode requires -pins 'Name=selector,...'")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	output, err := events.Open(cfg.LogDir)
	if err != nil {
		return err
	}
	defer output.Close()
	logger := log.New(output, "", log.LstdFlags|log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var client gateway.Client
	if cfg.Mode == "real" {
		client = real.New(cfg)
	} else {
		client = demo.New()
	}
	service, err := app.New(ctx, cfg, client, logger)
	if err != nil {
		return err
	}
	go service.Run(ctx)
	server := &http.Server{Addr: cfg.Listen, Handler: httpapi.Handler(service), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	fmt.Printf("sing-box-smart · %s 模式 · http://%s\n", cfg.Mode, cfg.Listen)
	select {
	case err := <-failures:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

var _ = context.Background
var _ domain.Probe
