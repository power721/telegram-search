package api

import (
	"context"
	"database/sql"

	"github.com/gin-gonic/gin"

	"tg-search/internal/adminauth"
	"tg-search/internal/apikey"
	"tg-search/internal/channel"
	"tg-search/internal/config"
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
	APIKeyService    *apikey.Service
	Settings         *repository.SettingsRepository
	AdminAuth        *adminauth.Service
	RuntimeConfig    config.Config
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
	if h.deps.APIKeyService == nil && h.deps.APIKeys != nil && h.deps.Settings != nil {
		h.deps.APIKeyService = apikey.NewService(h.deps.APIKeys, h.deps.Settings)
	}
	api := router.Group("/api")
	api.GET("/health", h.health)
	api.GET("/ready", h.ready)
	api.GET("/setup/status", h.setupStatus)
	api.POST("/setup/admin", h.setupAdmin)
	api.POST("/setup/api-key", h.setupAPIKey)
	api.POST("/setup/telegram-api", h.saveSetupTelegramAPI)
	api.POST("/setup/listen-rules", h.setupListenRules)
	api.POST("/setup/complete", h.setupComplete)
	api.POST("/auth/login", h.authLogin)
	api.POST("/auth/logout", h.authLogout)
	api.GET("/auth/me", h.authMe)
	api.GET("/settings/telegram-api", h.getTelegramAPISettings)
	api.PUT("/settings/telegram-api", h.updateTelegramAPISettings)
	api.GET("/settings/api-key", h.getAPIKeySettings)
	api.POST("/settings/api-key/regenerate", h.regenerateAPIKey)

	business := api.Group("")
	business.Use(h.requireAPIKey())
	business.GET("/storage/usage", h.storageUsage)
	business.GET("/status", h.status)
	business.GET("/tasks", h.tasks)
	business.GET("/tasks/:id", h.task)
	business.POST("/tasks/:id/retry", h.retryTask)
	business.POST("/tasks/:id/cancel", h.cancelTask)
	business.POST("/tasks/:id/pause", h.pauseTask)
	business.POST("/tasks/:id/resume", h.resumeTask)
	business.GET("/events", h.events)
	telegramAPI := business.Group("/telegram")
	telegramAPI.POST("/login/send-code", h.sendCode)
	telegramAPI.POST("/login/sign-in", h.signIn)
	telegramAPI.POST("/login/password", h.password)
	business.GET("/accounts", h.accounts)
	business.POST("/accounts/:id/logout", h.logoutAccount)
	business.DELETE("/accounts/:id", h.deleteAccount)
	business.POST("/accounts/:id/channels/sync-metadata", h.syncAccountChannels)
	business.GET("/channels", h.channels)
	business.POST("/channels/sync", h.syncChannels)
	business.POST("/channels/web-access/check", h.checkChannelWebAccess)
	business.PATCH("/channels/control", h.updateChannelsControl)
	business.GET("/channels/:id", h.channel)
	business.PATCH("/channels/:id/control", h.updateChannelControl)
	business.POST("/channels/:id/analyze", h.analyzeChannel)
	business.POST("/channels/:id/sync", h.syncChannel)
	business.GET("/watch-rules", h.watchRules)
	business.POST("/watch-rules", h.createWatchRule)
	business.GET("/watch-rules/:id", h.watchRule)
	business.PUT("/watch-rules/:id", h.updateWatchRule)
	business.DELETE("/watch-rules/:id", h.deleteWatchRule)
	business.GET("/search/global", h.searchGlobal)
	business.GET("/search/messages", h.searchMessages)
	business.GET("/search/links", h.searchLinks)
	business.GET("/search/files", h.searchFiles)
	business.GET("/search/channels", h.searchChannels)
	business.GET("/search", h.search)
	business.POST("/search/remote", h.createRemoteSearchTask)
	business.GET("/search/remote/:task_id", h.getRemoteSearchTask)
	business.GET("/messages/latest", h.latest)
	business.GET("/links/merged", h.mergedLinks)
	business.GET("/links", h.links)
	business.GET("/resources/grouped", h.resourcesGrouped)
	business.GET("/resources/:id", h.resource)
	business.GET("/resources", h.resources)
	business.POST("/maintenance/sqlite", h.maintenanceSQLite)
	business.POST("/maintenance/backup", h.maintenanceBackup)

	router.NoRoute(h.frontend)
	return router
}
