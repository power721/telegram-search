package api

import (
	"context"
	"database/sql"

	"github.com/gin-gonic/gin"

	"tg-search/internal/adminauth"
	"tg-search/internal/channel"
	"tg-search/internal/history"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
	"tg-search/internal/scheduler"
	"tg-search/internal/search"
	"tg-search/internal/session"
	"tg-search/internal/storage"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
)

type AccountRuntime interface {
	StopAccount(context.Context, int64) error
}

type Dependencies struct {
	Users            *repository.UserRepository
	APIKeys          *repository.APIKeyRepository
	Settings         *repository.SettingsRepository
	AdminAuth        *adminauth.Service
	StorageUsage     *storage.UsageService
	Accounts         *repository.AccountRepository
	Channels         *repository.ChannelRepository
	Messages         *repository.MessageRepository
	Links            *repository.LinkRepository
	WatchRules       *repository.WatchRuleRepository
	RemoteSearch     *repository.RemoteSearchTaskRepository
	RemoteSearchExec *search.RemoteService
	Maintenance      *repository.MaintenanceRepository
	Status           *repository.StatusRepository
	BackupDB         *sql.DB
	BackupDir        string
	SyncQueue        *scheduler.RetryQueue
	Search           *search.Service
	History          *history.Service
	Resources        *resource.Service
	ChannelSync      *channel.Service
	ChannelWebAccess *channel.WebAccessService
	Tasks            *taskpkg.Service
	TaskRepository   *taskpkg.Repository
	Events           *taskpkg.EventBroker
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
	api.GET("/setup/status", h.setupStatus)
	api.POST("/setup/admin", h.setupAdmin)
	api.POST("/setup/api-key", h.setupAPIKey)
	api.POST("/setup/telegram-api", h.saveSetupTelegramAPI)
	api.POST("/setup/complete", h.setupComplete)
	api.POST("/auth/login", h.authLogin)
	api.POST("/auth/logout", h.authLogout)
	api.GET("/auth/me", h.authMe)
	api.GET("/settings/telegram-api", h.getTelegramAPISettings)
	api.PUT("/settings/telegram-api", h.updateTelegramAPISettings)
	api.GET("/storage/usage", h.storageUsage)
	api.GET("/status", h.status)
	api.GET("/tasks", h.tasks)
	api.GET("/tasks/:id", h.task)
	api.POST("/tasks/:id/retry", h.retryTask)
	api.POST("/tasks/:id/cancel", h.cancelTask)
	api.POST("/tasks/:id/pause", h.pauseTask)
	api.POST("/tasks/:id/resume", h.resumeTask)
	api.GET("/events", h.events)
	telegramAPI := api.Group("/telegram")
	telegramAPI.POST("/login/send-code", h.sendCode)
	telegramAPI.POST("/login/sign-in", h.signIn)
	telegramAPI.POST("/login/password", h.password)
	api.GET("/accounts", h.accounts)
	api.DELETE("/accounts/:id", h.deleteAccount)
	api.POST("/accounts/:id/channels/sync-metadata", h.syncAccountChannels)
	api.GET("/channels", h.channels)
	api.POST("/channels/sync", h.syncChannels)
	api.POST("/channels/web-access/check", h.checkChannelWebAccess)
	api.GET("/channels/:id", h.channel)
	api.PATCH("/channels/:id/control", h.updateChannelControl)
	api.POST("/channels/:id/analyze", h.analyzeChannel)
	api.POST("/channels/:id/sync", h.syncChannel)
	api.GET("/watch-rules", h.watchRules)
	api.POST("/watch-rules", h.createWatchRule)
	api.GET("/watch-rules/:id", h.watchRule)
	api.PUT("/watch-rules/:id", h.updateWatchRule)
	api.DELETE("/watch-rules/:id", h.deleteWatchRule)
	api.GET("/search/global", h.searchGlobal)
	api.GET("/search/messages", h.searchMessages)
	api.GET("/search/links", h.searchLinks)
	api.GET("/search/files", h.searchFiles)
	api.GET("/search/channels", h.searchChannels)
	api.GET("/search", h.search)
	api.POST("/search/remote", h.createRemoteSearchTask)
	api.GET("/search/remote/:task_id", h.getRemoteSearchTask)
	api.GET("/messages/latest", h.latest)
	api.GET("/links/merged", h.mergedLinks)
	api.GET("/links", h.links)
	api.GET("/resources/grouped", h.resourcesGrouped)
	api.GET("/resources/:id", h.resource)
	api.GET("/resources", h.resources)
	api.POST("/maintenance/sqlite", h.maintenanceSQLite)
	api.POST("/maintenance/backup", h.maintenanceBackup)

	return router
}
