package api

import (
	"context"
	"database/sql"
	"time"

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
	Files            *repository.FileRepository
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
	QRLogins         *QRLoginStore
}

func NewRouter(deps Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	h := handlers{deps: deps}
	if h.deps.APIKeyService == nil && h.deps.APIKeys != nil && h.deps.Settings != nil {
		h.deps.APIKeyService = apikey.NewService(h.deps.APIKeys, h.deps.Settings)
	}
	if h.deps.QRLogins == nil {
		h.deps.QRLogins = NewQRLoginStore(2 * time.Minute)
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
	api.PUT("/settings/admin", h.updateAdminSettings)
	api.GET("/settings/api-key", h.getAPIKeySettings)
	api.POST("/settings/api-key/regenerate", h.regenerateAPIKey)

	external := router.Group("")
	external.Use(h.requireAPIKey())
	external.GET("/api/search", h.externalSearch)
	external.POST("/api/search", h.externalSearch)

	adminOnly := api.Group("")
	adminOnly.Use(h.requireAdminSession())
	adminOnly.GET("/listen-rules", h.getListenRules)
	adminOnly.PUT("/listen-rules", h.updateListenRules)
	adminOnly.GET("/storage/usage", h.storageUsage)
	adminOnly.GET("/status", h.status)
	adminOnly.GET("/tasks", h.tasks)
	adminOnly.POST("/tasks/bulk-delete", h.bulkDeleteTasks)
	adminOnly.GET("/tasks/:id", h.task)
	adminOnly.DELETE("/tasks/:id", h.deleteTask)
	adminOnly.POST("/tasks/:id/retry", h.retryTask)
	adminOnly.POST("/tasks/:id/cancel", h.cancelTask)
	adminOnly.POST("/tasks/:id/pause", h.pauseTask)
	adminOnly.POST("/tasks/:id/resume", h.resumeTask)
	adminOnly.GET("/events", h.events)
	adminOnly.GET("/logs", h.logs)
	adminOnly.GET("/logs/:file/download", h.downloadLog)
	telegramAPI := adminOnly.Group("/telegram")
	telegramAPI.POST("/login/send-code", h.sendCode)
	telegramAPI.POST("/login/sign-in", h.signIn)
	telegramAPI.POST("/login/password", h.password)
	telegramAPI.POST("/login/qr/start", h.startQRLogin)
	telegramAPI.GET("/login/qr/:login_id", h.pollQRLogin)
	telegramAPI.DELETE("/login/qr/:login_id", h.cancelQRLogin)
	adminOnly.GET("/accounts", h.accounts)
	adminOnly.POST("/accounts/:id/logout", h.logoutAccount)
	adminOnly.DELETE("/accounts/:id", h.deleteAccount)
	adminOnly.POST("/accounts/:id/channels/sync-metadata", h.syncAccountChannels)
	adminOnly.GET("/channels", h.channels)
	adminOnly.POST("/channels/sync", h.syncChannels)
	adminOnly.POST("/channels/web-access/check", h.checkChannelWebAccess)
	adminOnly.PATCH("/channels/control", h.updateChannelsControl)
	adminOnly.GET("/channels/:id", h.channel)
	adminOnly.PATCH("/channels/:id/control", h.updateChannelControl)
	adminOnly.POST("/channels/:id/analyze", h.analyzeChannel)
	adminOnly.POST("/channels/:id/sync", h.syncChannel)
	adminOnly.GET("/watch-rules", h.watchRules)
	adminOnly.POST("/watch-rules", h.createWatchRule)
	adminOnly.GET("/watch-rules/:id", h.watchRule)
	adminOnly.PUT("/watch-rules/:id", h.updateWatchRule)
	adminOnly.DELETE("/watch-rules/:id", h.deleteWatchRule)
	adminSearch := adminOnly.Group("/admin/search")
	adminSearch.GET("/global", h.searchGlobal)
	adminSearch.GET("/messages", h.searchMessages)
	adminSearch.GET("/links", h.searchLinks)
	adminSearch.GET("/files", h.searchFiles)
	adminSearch.GET("/channels", h.searchChannels)
	adminSearch.GET("", h.search)
	adminSearch.POST("/remote", h.createRemoteSearchTask)
	adminSearch.GET("/remote/:task_id", h.getRemoteSearchTask)
	adminOnly.GET("/messages/latest", h.latest)
	adminOnly.GET("/links/merged", h.mergedLinks)
	adminOnly.GET("/links/grouped", h.linksGrouped)
	adminOnly.GET("/links", h.links)
	adminOnly.POST("/maintenance/sqlite", h.maintenanceSQLite)
	adminOnly.POST("/maintenance/backup", h.maintenanceBackup)

	resourceAccess := api.Group("")
	resourceAccess.Use(h.requireAdminSession())
	resourceAccess.GET("/resources/grouped", h.resourcesGrouped)
	resourceAccess.GET("/resources/:id", h.resource)
	resourceAccess.GET("/resources", h.resources)

	mediaAccess := router.Group("")
	mediaAccess.Use(h.requireMediaAccess())
	mediaAccess.GET("/v/:fileid", h.serveTelegramVideo)
	mediaAccess.HEAD("/v/:fileid", h.serveTelegramVideo)
	mediaAccess.GET("/i/:fileid", h.serveTelegramImage)
	mediaAccess.HEAD("/i/:fileid", h.serveTelegramImage)

	router.NoRoute(h.frontend)
	return router
}
