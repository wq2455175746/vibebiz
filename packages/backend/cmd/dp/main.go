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

	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/clone"
	"vibebiz/internal/config"
	"vibebiz/internal/dao"
	"vibebiz/internal/handler"
	"vibebiz/internal/observability"
	"vibebiz/internal/resource"
	"vibebiz/internal/role"
	"vibebiz/internal/router"
	storages3 "vibebiz/internal/s3"
	"vibebiz/internal/sandboxregistry"
	"vibebiz/internal/server"
	"vibebiz/internal/storagesync"
	"vibebiz/internal/store"
	"vibebiz/internal/usage"
	"vibebiz/internal/usertrace"
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

	sessions := auth.NewRedisStore(redisClient, redisPrefix)
	userCookie := auth.CookieManager{Name: "vibebiz_user", Secure: cfg.CookieSecure, TTL: cfg.SessionUserTTL, Now: time.Now}
	authService := auth.NewService(nil, nil, sessions, cfg.SessionUserTTL, cfg.SessionOpsTTL)
	authn := auth.NewAuthenticator(authService, userCookie, auth.CookieManager{})
	metas := auth.NewProjectMetaStore(redisClient, redisPrefix)

	registryRepo := sandboxregistry.NewMySQLRepository(db)
	selector := sandboxregistry.NewSelector(registryRepo, cfg.SandboxHeartbeatTTL, time.Now)
	resolver := sandboxregistry.NewResolver(registryRepo, selector, cfg.SandboxHeartbeatTTL, cfg.SandboxLeaseTTL, time.Now)
	resolver.SetMetrics(obs)
	sandbox := client.NewLeasedRouter(resolver, resolver, obs, cfg.SandboxLeaseRenewInterval)

	var resourceS3 resource.S3
	var coverS3 handler.CoverS3
	if cfg.S3Bucket != "" {
		presigner, err := storages3.NewPresigner(ctx, storages3.Config{
			Bucket:          cfg.S3Bucket,
			Region:          cfg.S3Region,
			Endpoint:        cfg.S3Endpoint,
			AccessKey:       cfg.S3AccessKey,
			SecretKey:       cfg.S3SecretKey,
			RemoteDirPrefix: cfg.S3RemoteDirPrefix,
			ForcePathStyle:  cfg.S3ForcePathStyle,
		})
		if err != nil {
			obs.Logger.Error("s3 presigner init failed", "error", err)
		} else {
			resourceS3 = presigner
			coverS3 = presigner
		}
	}
	usageLoc, err := time.LoadLocation(cfg.UsageTimezone)
	if err != nil {
		obs.Logger.Warn("invalid usage timezone, falling back to UTC", "value", cfg.UsageTimezone)
		usageLoc = time.UTC
	}
	usageService := usage.NewService(usage.NewMySQLRepository(db), usageLoc)
	resourceService := resource.NewService(dao.NewProjectResourceRepository(db), resourceS3, sandbox)
	resourceService.Usage = usageService
	resourceService.Logger = obs.Logger
	resourceService.Expiry = cfg.S3PresignExpiry()
	resourceHandler := handler.NewResourceHandler(resourceService, obs)
	cloneService := clone.NewService(nil, nil, nil, nil, nil, nil, resourceService, clone.NewMySQLStore(db))
	cloneHandler := handler.NewCloneHandler(cloneService, obs)
	cloneHandler.Tests = sandbox

	projectStore, err := store.NewMySQLProjectStore(ctx, db)
	if err != nil {
		obs.Logger.Error("project store init failed", "error", err)
		os.Exit(1)
	}
	cloneService.Projects = projectStore
	cloneService.SandboxClone = sandbox
	roleService := agentrole.NewService(agentrole.NewMySQLRepository(db), nil, 0)
	roleResolver := role.NewResolver(projectStore, roleService)
	storageSyncer := &storagesync.Syncer{
		Projects: projectStore,
		Sandbox:  sandbox,
		Repo:     storagesync.NewMySQLRepository(db),
		Logger:   obs.Logger,
	}

	coverHandler := handler.NewCoverHandler(projectStore, coverS3, cfg.S3PresignExpiry())

	traceRepo := usertrace.NewMySQLRepository(db)
	var traceRecorder *usertrace.Recorder
	if cfg.UserTraceEnabled {
		if err := usertrace.EnsureWritable(cfg.UserTraceDir); err != nil {
			obs.Logger.Error("user trace dir unavailable; tracing disabled", "dir", cfg.UserTraceDir, "error", err)
		} else {
			traceRecorder = usertrace.NewRecorder(cfg.UserTraceDir, usertrace.TrimPlane(cfg.Plane), obs.Logger)
			traceCtx, traceCancel := context.WithCancel(ctx)
			defer traceCancel()
			go usertrace.NewSyncer(cfg.UserTraceDir, usertrace.TrimPlane(cfg.Plane), traceRepo, cfg.UserTraceSyncInterval, obs.Logger).Run(traceCtx)
		}
	}

	e := server.New(cfg, obs)
	if traceRecorder != nil {
		e.Use(usertrace.Middleware(traceRecorder, obs.Logger))
	}
	router.RegisterData(e, sandbox, obs, authn, metas, resourceHandler, roleResolver, coverHandler, storageSyncer, cloneHandler, usageService)

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	go func() {
		obs.Logger.Info("dp listening", "host", cfg.Host, "port", cfg.Port, "sandbox_registry", "mysql")
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
