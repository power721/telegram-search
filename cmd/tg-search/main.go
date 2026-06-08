package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.uber.org/zap"

	"tg-search/internal/account"
	"tg-search/internal/adminauth"
	"tg-search/internal/api"
	"tg-search/internal/channel"
	"tg-search/internal/config"
	"tg-search/internal/db"
	"tg-search/internal/history"
	"tg-search/internal/link"
	"tg-search/internal/logger"
	"tg-search/internal/messagefilter"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
	"tg-search/internal/retry"
	"tg-search/internal/scheduler"
	"tg-search/internal/search"
	"tg-search/internal/session"
	"tg-search/internal/storage"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
	updatepkg "tg-search/internal/update"
)

func main() {
	configPath := flag.String("config", "", "config file path")
	flag.Parse()

	if err := run(*configPath); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := config.EnsureRuntimeDirs(cfg); err != nil {
		return err
	}

	logs, err := logger.New(filepath.Join(cfg.Storage.Path, "logs"))
	if err != nil {
		return err
	}
	defer logs.Sync()
	logs.App.Info("tg-search starting", zap.String("address", config.Address(cfg)))

	conn, err := db.Open(config.DatabasePath(cfg))
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}

	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	messages := repository.NewMessageRepository(conn)
	links := repository.NewLinkRepository(conn)
	files := repository.NewFileRepository(conn)
	cursors := repository.NewSyncCursorRepository(conn)
	watchRules := repository.NewWatchRuleRepository(conn)
	remoteSearch := repository.NewRemoteSearchTaskRepository(conn)
	watchFilter := messagefilter.New(watchRules)
	maintenance := repository.NewMaintenanceRepository(conn)
	status := repository.NewStatusRepository(conn)
	users := repository.NewUserRepository(conn)
	apiKeys := repository.NewAPIKeyRepository(conn)
	settings := repository.NewSettingsRepository(conn)
	taskRepository := taskpkg.NewRepository(conn)
	taskService := taskpkg.NewService(taskRepository)
	eventBroker := taskpkg.NewEventBroker()
	adminAuth := adminauth.NewService(users)
	storageUsage := storage.NewUsageService(cfg)
	sessions := session.NewManager(filepath.Join(cfg.Storage.Path, "sessions"))
	tgClient := telegram.NewGotdClient(cfg.Telegram.APIID, cfg.Telegram.APIHash, logs.Telegram)
	retryPolicy := retry.DefaultPolicy()
	syncQueue := scheduler.NewRetryQueue(scheduler.RetryQueueOptions{Policy: retryPolicy, Logger: logs.SyncLog})
	updateProcessor := updatepkg.NewProcessor(updatepkg.ProcessorOptions{
		DB:        conn,
		Channels:  channels,
		Messages:  messages,
		Links:     links,
		Cursors:   cursors,
		Tasks:     taskService,
		Extractor: link.NewExtractor(),
		Filter:    watchFilter,
	})
	updateService := updatepkg.NewService(updatepkg.ServiceOptions{
		Accounts:    accounts,
		Channels:    channels,
		Processor:   updateProcessor,
		Listener:    updatepkg.NewGotdListener(cfg.Telegram.APIID, cfg.Telegram.APIHash, sessions, logs.Telegram),
		RetryPolicy: retryPolicy,
		Logger:      logs.Telegram,
	})
	searchService := search.NewService(messages, links, files, channels)
	resourceService := resource.NewService(links, files)
	remoteSearchService := search.NewRemoteService(search.RemoteOptions{
		Accounts: accounts,
		Channels: channels,
		Tasks:    remoteSearch,
		Cursors:  cursors,
		Telegram: tgClient,
		Sessions: sessions,
	})
	historyService := history.NewService(history.Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links, Cursors: cursors,
		Telegram: tgClient, Sessions: sessions, Extractor: link.NewExtractor(),
		Filter:           watchFilter,
		HistoryBatchSize: cfg.Sync.HistoryBatchSize,
		Workers:          cfg.Sync.Workers,
		RetryPolicy:      retryPolicy,
	})
	channelService := channel.NewService(channels, tgClient, sessions)
	channelWebAccessService := channel.NewWebAccessService(channels, nil)
	if err := taskService.RestoreUnfinished(ctx, time.Now().UTC()); err != nil {
		return err
	}
	accountManager := account.NewManager(account.ManagerOptions{
		Accounts: accounts,
		Updates:  updateService,
		Logger:   logs.Telegram,
	})
	if err := accountManager.Start(ctx); err != nil {
		return err
	}
	cleanupScheduler := scheduler.New(scheduler.Options{
		Interval: time.Hour,
		Jobs: []scheduler.Job{
			scheduler.CleanupJob{Logger: logs.App},
		},
		Logger: logs.App,
	})
	cleanupScheduler.Start(ctx)

	router := api.NewRouter(api.Dependencies{
		Users: users, APIKeys: apiKeys, Settings: settings, AdminAuth: adminAuth, RuntimeConfig: cfg, StorageUsage: storageUsage,
		Accounts: accounts, Channels: channels, Messages: messages, Links: links, WatchRules: watchRules, RemoteSearch: remoteSearch, RemoteSearchExec: remoteSearchService, Maintenance: maintenance, Status: status,
		BackupDB: conn, BackupDir: filepath.Join(cfg.Storage.Path, "backup"),
		SyncQueue: syncQueue, Search: searchService, History: historyService, Resources: resourceService, ChannelSync: channelService, ChannelWebAccess: channelWebAccessService, AccountRuntime: accountManager,
		Tasks: taskService, TaskRepository: taskRepository, Events: eventBroker,
		Telegram: tgClient, Sessions: sessions, CodeStore: telegram.NewCodeStore(),
	})
	server := &http.Server{
		Addr:              config.Address(cfg),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logs.App.Info("api server listening", zap.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-stop:
		logs.App.Info("shutdown requested", zap.String("signal", sig.String()))
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := cleanupScheduler.Stop(shutdownCtx); err != nil {
		return err
	}
	if err := syncQueue.Stop(shutdownCtx); err != nil {
		return err
	}
	if err := accountManager.Stop(shutdownCtx); err != nil {
		return err
	}
	logs.App.Info("tg-search stopped")
	return nil
}
