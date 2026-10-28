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
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/config"
	"vibebiz/internal/dao"
	"vibebiz/internal/handler"
	"vibebiz/internal/observability"
	"vibebiz/internal/router"
	"vibebiz/internal/server"
	"vibebiz/internal/store"
)

const redisPrefix = "vibebiz"

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

	db, err := dao.OpenMySQL(ctx, dao.MySQLConfig{
		Host:         cfg.MySQLHost,
		Username:     cfg.MySQLUsername,
		Password:     cfg.MySQLPassword,
		Database:     cfg.MySQLDatabase,
		MaxOpenConns: cfg.MySQLMaxOpenConns,
		MaxIdleConns: cfg.MySQLMaxIdleConns,
	})
	if err != nil {
		obs.Logger.Error("mysql init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := dao.CloseMySQL(db); err != nil {
			obs.Logger.Error("mysql close failed", "error", err)
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

	identity := dao.NewIdentityRepository(db)
	sessions := auth.NewRedisStore(redisClient, redisPrefix)
	userCookie := auth.CookieManager{Name: "vibebiz_user", Secure: cfg.CookieSecure, TTL: cfg.SessionUserTTL, Now: time.Now}
	opsCookie := auth.CookieManager{Name: "vibebiz_ops", Secure: cfg.CookieSecure, TTL: cfg.SessionOpsTTL, Now: time.Now}
	authService := auth.NewService(identity, identity, sessions, cfg.SessionUserTTL, cfg.SessionOpsTTL)
	authn := auth.NewAuthenticator(authService, userCookie, opsCookie)
	authHandler := &handler.AuthHandler{Service: authService, UserCookie: userCookie, OpsCookie: opsCookie}
	metas := auth.NewProjectMetaStore(redisClient, redisPrefix)

	projectStore := store.NewMemoryProjectStore()
	sandbox := client.NewSandbox(cfg.SandboxBaseURL, obs)

	e := server.New(cfg, obs)
	router.RegisterControl(e, admin.NewMemoryRepository(), projectStore, sandbox, obs, authn, authHandler, metas)

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
