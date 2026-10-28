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

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/config"
	"vibebiz/internal/observability"
	"vibebiz/internal/router"
	"vibebiz/internal/server"
)

const redisPrefix = "vibebiz"

func main() {
	cfg := config.Load("dp")

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

	redisClient := auth.NewRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		obs.Logger.Error("redis init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			obs.Logger.Error("redis close failed", "error", err)
		}
	}()

	sessions := auth.NewRedisStore(redisClient, redisPrefix)
	userCookie := auth.CookieManager{Name: "vibebiz_user", Secure: cfg.CookieSecure, TTL: cfg.SessionUserTTL, Now: time.Now}
	authService := auth.NewService(nil, nil, sessions, cfg.SessionUserTTL, cfg.SessionOpsTTL)
	authn := auth.NewAuthenticator(authService, userCookie, auth.CookieManager{})
	metas := auth.NewProjectMetaStore(redisClient, redisPrefix)

	sandbox := client.NewSandbox(cfg.SandboxBaseURL, obs)

	e := server.New(cfg, obs)
	router.RegisterData(e, sandbox, obs, authn, metas)

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	go func() {
		obs.Logger.Info("dp listening", "host", cfg.Host, "port", cfg.Port, "sandbox", cfg.SandboxBaseURL)
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
