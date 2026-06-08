package api

import (
	"context"
	"database/sql"

	"github.com/gin-gonic/gin"

	"tg-search/internal/channel"
	"tg-search/internal/history"
	"tg-search/internal/repository"
	"tg-search/internal/scheduler"
	"tg-search/internal/search"
	"tg-search/internal/session"
	"tg-search/internal/telegram"
)

type AccountRuntime interface {
	StopAccount(context.Context, int64) error
}

type Dependencies struct {
	Accounts         *repository.AccountRepository
	Channels         *repository.ChannelRepository
	Messages         *repository.MessageRepository
	Links            *repository.LinkRepository
	WatchRules       *repository.WatchRuleRepository
	Maintenance      *repository.MaintenanceRepository
	Status           *repository.StatusRepository
	BackupDB         *sql.DB
	BackupDir        string
	SyncQueue        *scheduler.RetryQueue
	Search           *search.Service
	History          *history.Service
	ChannelSync      *channel.Service
	ChannelWebAccess *channel.WebAccessService
	AccountRuntime   AccountRuntime
	Telegram         telegram.Client
	Sessions         *session.Manager
	CodeStore        *telegram.CodeStore
}

func NewRouter(deps Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	h := handlers{deps: deps}
	api := router.Group("/api")
	api.GET("/status", h.status)
	api.POST("/login/send-code", h.sendCode)
	api.POST("/login/sign-in", h.signIn)
	api.POST("/login/password", h.password)
	api.GET("/accounts", h.accounts)
	api.DELETE("/accounts/:id", h.deleteAccount)
	api.POST("/accounts/:id/channels/sync", h.syncAccountChannels)
	api.GET("/channels", h.channels)
	api.POST("/channels/sync", h.syncChannels)
	api.POST("/channels/web-access/check", h.checkChannelWebAccess)
	api.GET("/channels/:id", h.channel)
	api.POST("/channels/:id/sync", h.syncChannel)
	api.GET("/watch-rules", h.watchRules)
	api.POST("/watch-rules", h.createWatchRule)
	api.GET("/watch-rules/:id", h.watchRule)
	api.PUT("/watch-rules/:id", h.updateWatchRule)
	api.DELETE("/watch-rules/:id", h.deleteWatchRule)
	api.GET("/search", h.search)
	api.GET("/messages/latest", h.latest)
	api.GET("/links/merged", h.mergedLinks)
	api.GET("/links", h.links)
	api.POST("/maintenance/sqlite", h.maintenanceSQLite)
	api.POST("/maintenance/backup", h.maintenanceBackup)

	return router
}
