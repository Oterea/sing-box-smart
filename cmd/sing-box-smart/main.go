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
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/httpapi"
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
	flag.StringVar(&cfg.Listen, "listen", cfg.Listen, "loopback HTTP listen address")
	flag.StringVar(&cfg.LogDir, "log-dir", cfg.LogDir, "bounded JSONL log directory")
	flag.Parse()
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
	service, err := app.New(ctx, cfg, demo.New(), logger)
	if err != nil {
		return err
	}
	go service.Run(ctx)
	server := &http.Server{Addr: cfg.Listen, Handler: httpapi.Handler(service), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	fmt.Printf("sing-box-smart · 模拟模式 · http://%s\n", cfg.Listen)
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
