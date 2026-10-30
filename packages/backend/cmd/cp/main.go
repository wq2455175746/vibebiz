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

	"github.com/hibiken/asynq"

	"vibebiz/internal/account"
	"vibebiz/internal/admin"
	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/config"
	"vibebiz/internal/dao"
	"vibebiz/internal/handler"
	harnesssvc "vibebiz/internal/harness"
	"vibebiz/internal/llmmodel"
	"vibebiz/internal/notification"
	"vibebiz/internal/notificationchannel"
	"vibebiz/internal/observability"
	"vibebiz/internal/projectstage"
	"vibebiz/internal/projecttemplate"
	"vibebiz/internal/requestcount"
	"vibebiz/internal/router"
	storages3 "vibebiz/internal/s3"
	"vibebiz/internal/sandboxregistry"
	"vibebiz/internal/secret"
	"vibebiz/internal/server"
	"vibebiz/internal/storagesync"
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
	metas := auth.NewProjectMetaStore(redisClient, redisPrefix)

	var avatarPresigner account.Presigner
	var s3Presigner *storages3.Presigner
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
			avatarPresigner = presigner
			s3Presigner = presigner
		}
	}
	settingsRepo := dao.NewSettingsRepository(db)
	accountService := account.NewService(settingsRepo, sessions, avatarPresigner, cfg.S3PresignExpiry())
	authHandler := &handler.AuthHandler{Service: authService, Account: accountService, UserCookie: userCookie, OpsCookie: opsCookie}
	settingsHandler := handler.NewSettingsHandler(accountService, userCookie)

	projectStore, err := store.NewMySQLProjectStore(ctx, db)
	if err != nil {
		obs.Logger.Error("project store init failed", "error", err)
		os.Exit(1)
	}
	stageStore := projectstage.NewMySQLStore(db)
	requestCounter := requestcount.NewCounter(redisClient, redisPrefix)
	adminRepo := admin.NewMySQLRepository(db, requestCounter)
	var modelCipher *secret.Cipher
	if cfg.ModelSecretKey != "" {
		modelCipher, err = secret.NewCipher(cfg.ModelSecretKey)
		if err != nil {
			obs.Logger.Error("model secret key invalid", "error", err)
			os.Exit(1)
		}
	}
	modelService := llmmodel.NewService(llmmodel.NewMySQLRepository(db), modelCipher)
	modelHandler := handler.NewModelHandler(modelService)
	roleService := agentrole.NewService(agentrole.NewMySQLRepository(db), avatarPresigner, cfg.S3PresignExpiry())
	roleHandler := handler.NewRoleHandler(roleService)
	registryRepo := sandboxregistry.NewMySQLRepository(db)
	selector := sandboxregistry.NewSelector(registryRepo, cfg.SandboxHeartbeatTTL, time.Now)
	resolver := sandboxregistry.NewResolver(registryRepo, selector, cfg.SandboxHeartbeatTTL, time.Now)
	provisioner := sandboxregistry.NewProvisioner(selector, sandboxregistry.NewHTTPInitializer(obs), 3)
	sandboxRouter := client.NewRouter(resolver, obs)
	var harnessImporter handler.HarnessImporter
	if s3Presigner != nil {
		harnessImporter = harnesssvc.NewService(s3Presigner, sandboxRouter, cfg.EnterpriseHarnessS3Key)
	}
	registryHandler := handler.NewSandboxRegistryHandler(registryRepo, cfg.SandboxRegistryToken, obs)
	storageHandler := &handler.StorageHandler{Projects: projectStore, Storage: storagesync.NewMySQLRepository(db), Account: accountService}

	notificationRepo := dao.NewNotificationRepository(db)
	notificationCache := notification.NewCache(redisClient, redisPrefix)
	notificationHub := notification.NewHub()
	notificationBus := notification.NewEventBus(redisClient, redisPrefix, notificationHub)
	busCtx, busCancel := context.WithCancel(ctx)
	go func() {
		if err := notificationBus.Run(busCtx); err != nil && !errors.Is(err, context.Canceled) {
			obs.Logger.Error("notification event bus stopped", "error", err)
		}
	}()
	select {
	case <-notificationBus.Ready():
	case <-time.After(5 * time.Second):
		obs.Logger.Error("notification event bus subscribe timeout")
	}
	notificationService := notification.NewService(notificationRepo, notificationCache, notificationBus)
	notificationHandler := handler.NewNotificationHandler(notificationService, notificationHub, cfg.CorsOrigins)

	var channelCipher *secret.Cipher
	if cfg.NotificationSecretKey != "" {
		channelCipher, err = secret.NewCipher(cfg.NotificationSecretKey)
		if err != nil {
			obs.Logger.Error("notification secret key invalid", "error", err)
			os.Exit(1)
		}
	} else {
		obs.Logger.Warn("notification secret key is empty; notification channels are disabled")
	}
	channelService := &notificationchannel.Service{
		Repo:   notificationchannel.NewMySQLRepository(db),
		Cipher: channelCipher,
		Sender: &notificationchannel.TestSender{Mailer: notificationchannel.NewSMTPMailer(notificationchannel.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword, From: cfg.SMTPFrom, Security: cfg.SMTPSecurity,
		})},
	}
	notificationChannelHandler := handler.NewNotificationChannelHandler(channelService)

	healthChecker := sandboxregistry.NewHealthChecker(registryRepo, cfg.SandboxHeartbeatTTL, cfg.SandboxHealthTimeout, obs)
	asynqLogger := sandboxregistry.NewAsyncLogger(obs.Logger)
	asynqRedis := asynq.RedisClientOpt{Addr: cfg.RedisHost, Password: cfg.RedisPassword, DB: cfg.AsynqRedisDB}
	asynqWorker := asynq.NewServer(asynqRedis, asynq.Config{Concurrency: cfg.AsynqConcurrency, Logger: asynqLogger})
	asynqMux := asynq.NewServeMux()
	asynqMux.HandleFunc(sandboxregistry.TaskTypeHealthCheck, healthChecker.HandleTask)
	asynqScheduler := asynq.NewScheduler(asynqRedis, &asynq.SchedulerOpts{Logger: asynqLogger})
	if _, err := asynqScheduler.Register("@every "+cfg.SandboxHealthInterval.String(), asynq.NewTask(sandboxregistry.TaskTypeHealthCheck, nil), asynq.MaxRetry(0), asynq.Timeout(15*time.Second), asynq.Unique(cfg.SandboxHealthInterval)); err != nil {
		obs.Logger.Error("sandbox health scheduler register failed", "error", err)
		os.Exit(1)
	}
	if err := asynqScheduler.Start(); err != nil {
		obs.Logger.Error("sandbox health scheduler start failed", "error", err)
		os.Exit(1)
	}
	defer asynqScheduler.Shutdown()
	if err := asynqWorker.Start(asynqMux); err != nil {
		obs.Logger.Error("sandbox health worker start failed", "error", err)
		os.Exit(1)
	}
	defer asynqWorker.Shutdown()

	e := server.New(cfg, obs)
	e.Use(requestCounter.Middleware())
	templateService := projecttemplate.NewService(projecttemplate.NewMySQLRepository(db), projectStore, sandboxRouter, avatarPresigner, cfg.S3PresignExpiry())
	templateService.Logger = obs.Logger
	templateHandler := handler.NewTemplateHandler(templateService, projectStore, provisioner, sandboxRouter)
	templateHandler.Metas = metas

	router.RegisterControl(e, adminRepo, projectStore, provisioner, obs, authn, authHandler, settingsHandler, storageHandler, metas, notificationHandler, registryHandler, modelHandler, roleHandler, avatarPresigner, cfg.S3PresignExpiry(), sandboxRouter, templateHandler, harnessImporter, stageStore, sandboxRouter, notificationChannelHandler)
	router.RegisterChatHistory(e, handler.NewChatHistoryHandler(sandboxRouter, projectStore), authn)

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	go func() {
		obs.Logger.Info("cp listening", "host", cfg.Host, "port", cfg.Port, "sandbox_registry", "mysql")
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			obs.Logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	busCancel()
	notificationHub.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		obs.Logger.Error("shutdown failed", "error", err)
	}
}
