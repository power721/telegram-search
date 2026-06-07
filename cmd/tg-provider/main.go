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

	"tg-provider/internal/api"
	"tg-provider/internal/channel"
	"tg-provider/internal/config"
	"tg-provider/internal/db"
	"tg-provider/internal/history"
	"tg-provider/internal/link"
	"tg-provider/internal/logger"
	"tg-provider/internal/repository"
	"tg-provider/internal/search"
	"tg-provider/internal/session"
	"tg-provider/internal/telegram"
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
	logs.App.Info("tg-provider starting", zap.String("address", config.Address(cfg)))

	conn, err := db.Open(filepath.Join(cfg.Storage.Path, "telegram.db"))
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
	status := repository.NewStatusRepository(conn)
	sessions := session.NewManager(filepath.Join(cfg.Storage.Path, "sessions"))
	tgClient := telegram.NewGotdClient(cfg.Telegram.APIID, cfg.Telegram.APIHash, logs.Telegram)
	searchService := search.NewService(messages, links)
	historyService := history.NewService(history.Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: tgClient, Sessions: sessions, Extractor: link.NewExtractor(),
		HistoryBatchSize: cfg.Sync.HistoryBatchSize,
	})
	channelService := channel.NewService(channels, tgClient, sessions)

	router := api.NewRouter(api.Dependencies{
		Accounts: accounts, Channels: channels, Messages: messages, Links: links, Status: status,
		Search: searchService, History: historyService, ChannelSync: channelService,
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
	logs.App.Info("tg-provider stopped")
	return nil
}
