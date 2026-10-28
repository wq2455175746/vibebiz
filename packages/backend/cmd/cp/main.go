package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"vibebiz/internal/admin"
	"vibebiz/internal/client"
	"vibebiz/internal/config"
	"vibebiz/internal/observability"
	"vibebiz/internal/router"
	"vibebiz/internal/server"
	"vibebiz/internal/store"
)

func main() {
	cfg := config.Load("cp")

	ctx := context.Background()
	obs, err := observability.New(ctx, cfg.ServiceName, cfg.Plane)
	if err != nil {
		fmt.Fprintf(os.Stderr, "observability init failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := obs.Shutdown(shutdownCtx); err != nil {
			obs.Logger.Error("observability shutdown failed", "error", err)
		}
	}()

	projectStore := store.NewMemoryProjectStore()
	sandbox := client.NewSandbox(cfg.SandboxBaseURL, obs)

	e := server.New(cfg, obs)
	router.RegisterControl(e, admin.NewMemoryRepository(), projectStore, sandbox, obs)

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	go func() {
		obs.Logger.Info("cp listening", "host", cfg.Host, "port", cfg.Port, "sandbox", cfg.SandboxBaseURL)
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			obs.Logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		obs.Logger.Error("shutdown failed", "error", err)
	}
}
