package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"tg-search/internal/adminauth"
	"tg-search/internal/backup"
	channelpkg "tg-search/internal/channel"
	"tg-search/internal/config"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
	searchsvc "tg-search/internal/search"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
	webui "tg-search/internal/web"
)

type handlers struct {
	deps Dependencies
}

const adminSessionCookie = "tg_search_session"
const (
	setupAPIKeyDoneKey      = "setup.api_key_done"
	setupListenRulesKey     = "setup.listen_rules"
	setupListenRulesDoneKey = "setup.listen_rules_done"
)

func (h handlers) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "ok",
	})
}

func (h handlers) ready(c *gin.Context) {
	checks := gin.H{}
	ready := true

	if h.deps.BackupDB == nil {
		ready = false
		checks["database"] = "missing"
	} else if err := h.deps.BackupDB.PingContext(c.Request.Context()); err != nil {
		ready = false
		checks["database"] = err.Error()
	} else {
		checks["database"] = "ok"
	}

	runtimeConfig := h.deps.RuntimeConfig
	if runtimeConfig.Storage.Path == "" && h.deps.StorageUsage != nil {
		runtimeConfig = h.deps.StorageUsage.Config()
	}
	if runtimeConfig.Storage.Path == "" {
		ready = false
		checks["runtime_dirs"] = "storage.path is required"
	} else if err := config.ValidateRuntimeDirs(runtimeConfig); err != nil {
		ready = false
		checks["runtime_dirs"] = err.Error()
	} else {
		checks["runtime_dirs"] = "ok"
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{
		"ready":  ready,
		"checks": checks,
	})
}

func (h handlers) setupStatus(c *gin.Context) {
	status, err := h.loadSetupStatus(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h handlers) frontend(c *gin.Context) {
	if c.Request.Method != http.MethodGet || strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.Status(http.StatusNotFound)
		return
	}
	dist, err := webui.Dist()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	data, err := readFrontendFile(dist, name)
	if err != nil {
		data, err = readFrontendFile(dist, "index.html")
		name = "index.html"
	}
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" || name == "index.html" {
		contentType = "text/html; charset=utf-8"
	}
	c.Data(http.StatusOK, contentType, data)
}

func readFrontendFile(dist fs.FS, name string) ([]byte, error) {
	info, err := fs.Stat(dist, name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fs.ErrInvalid
	}
	return fs.ReadFile(dist, name)
}

func (h handlers) setupAdmin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.deps.AdminAuth.CreateAdmin(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "username": strings.TrimSpace(req.Username), "role": model.UserRoleAdmin})
}

func (h handlers) setupAPIKey(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		errorText(c, http.StatusBadRequest, "name is required")
		return
	}
	plaintext := strings.ReplaceAll(uuid.NewString(), "-", "")
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	prefix := plaintext[:8]
	id, err := h.deps.APIKeys.Create(c.Request.Context(), model.APIKey{
		Name:    name,
		KeyHash: string(hash),
		Prefix:  prefix,
		Enabled: true,
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.deps.Settings.Set(c.Request.Context(), setupAPIKeyDoneKey, `true`); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "name": name, "prefix": prefix, "key": plaintext})
}

func (h handlers) skipSetupAPIKey(c *gin.Context) {
	if err := h.deps.Settings.Set(c.Request.Context(), setupAPIKeyDoneKey, `true`); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	status, err := h.loadSetupStatus(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h handlers) setupListenRules(c *gin.Context) {
	var req model.ListenRules
	if !bindJSON(c, &req) {
		return
	}
	rules := model.ListenRules{
		Includes:     normalizeSetupStrings(req.Includes),
		Excludes:     normalizeSetupStrings(req.Excludes),
		MessageTypes: normalizeSetupStrings(req.MessageTypes),
		LinkTypes:    normalizeSetupStrings(req.LinkTypes),
	}
	if len(rules.MessageTypes) == 0 {
		errorText(c, http.StatusBadRequest, "message_types is required")
		return
	}
	if len(rules.LinkTypes) == 0 {
		errorText(c, http.StatusBadRequest, "link_types is required")
		return
	}
	data, err := json.Marshal(rules)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.deps.Settings.Set(c.Request.Context(), setupListenRulesKey, string(data)); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.deps.Settings.Set(c.Request.Context(), setupListenRulesDoneKey, `true`); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	status, err := h.loadSetupStatus(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h handlers) saveSetupTelegramAPI(c *gin.Context) {
	settings, ok := readSetupTelegramAPISettingsRequest(c)
	if !ok {
		return
	}
	if settings.AppID == 0 && settings.AppHash == "" {
		runtimeSettings := model.TelegramAPISettings{
			AppID:   h.deps.RuntimeConfig.Telegram.APIID,
			AppHash: h.deps.RuntimeConfig.Telegram.APIHash,
		}
		c.JSON(http.StatusOK, repository.RedactTelegramAPI(runtimeSettings))
		return
	}
	if err := h.deps.Settings.SaveTelegramAPI(c.Request.Context(), settings); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, repository.RedactTelegramAPI(settings))
}

func (h handlers) setupComplete(c *gin.Context) {
	if err := h.deps.Settings.Set(c.Request.Context(), "setup.complete", `true`); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	status, err := h.loadSetupStatus(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h handlers) getTelegramAPISettings(c *gin.Context) {
	settings, err := h.deps.Settings.LoadTelegramAPI(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, repository.RedactTelegramAPI(settings))
}

func (h handlers) updateTelegramAPISettings(c *gin.Context) {
	settings, ok := readTelegramAPISettingsRequest(c, true)
	if !ok {
		return
	}
	if err := h.deps.Settings.SaveTelegramAPI(c.Request.Context(), settings); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, repository.RedactTelegramAPI(settings))
}

func readTelegramAPISettingsRequest(c *gin.Context, requireHash bool) (model.TelegramAPISettings, bool) {
	var req struct {
		AppID   int    `json:"app_id"`
		AppHash string `json:"app_hash"`
	}
	if !bindJSON(c, &req) {
		return model.TelegramAPISettings{}, false
	}
	if req.AppID <= 0 {
		errorText(c, http.StatusBadRequest, "app_id must be greater than zero")
		return model.TelegramAPISettings{}, false
	}
	req.AppHash = strings.TrimSpace(req.AppHash)
	if requireHash && req.AppHash == "" {
		errorText(c, http.StatusBadRequest, "app_hash is required")
		return model.TelegramAPISettings{}, false
	}
	return model.TelegramAPISettings{AppID: req.AppID, AppHash: req.AppHash}, true
}

func readSetupTelegramAPISettingsRequest(c *gin.Context) (model.TelegramAPISettings, bool) {
	var req struct {
		AppID   int    `json:"app_id"`
		AppHash string `json:"app_hash"`
	}
	if !bindJSON(c, &req) {
		return model.TelegramAPISettings{}, false
	}
	req.AppHash = strings.TrimSpace(req.AppHash)
	if req.AppID == 0 && req.AppHash == "" {
		return model.TelegramAPISettings{}, true
	}
	if req.AppID <= 0 {
		errorText(c, http.StatusBadRequest, "app_id must be greater than zero")
		return model.TelegramAPISettings{}, false
	}
	if req.AppHash == "" {
		errorText(c, http.StatusBadRequest, "app_hash is required")
		return model.TelegramAPISettings{}, false
	}
	return model.TelegramAPISettings{AppID: req.AppID, AppHash: req.AppHash}, true
}

func (h handlers) authLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	user, err := h.deps.AdminAuth.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, adminauth.ErrInvalidCredentials) {
			errorText(c, http.StatusUnauthorized, "invalid username or password")
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	token, err := h.deps.AdminAuth.CreateSession(user)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.SetCookie(adminSessionCookie, token, 86400, "/", "", false, true)
	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

func (h handlers) authLogout(c *gin.Context) {
	if cookie, err := c.Cookie(adminSessionCookie); err == nil {
		h.deps.AdminAuth.DeleteSession(cookie)
	}
	c.SetCookie(adminSessionCookie, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"logged_out": true})
}

func (h handlers) authMe(c *gin.Context) {
	cookie, err := c.Cookie(adminSessionCookie)
	if err != nil {
		errorText(c, http.StatusUnauthorized, "not authenticated")
		return
	}
	user, ok := h.deps.AdminAuth.UserForSession(cookie)
	if !ok {
		errorText(c, http.StatusUnauthorized, "not authenticated")
		return
	}
	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

func (h handlers) storageUsage(c *gin.Context) {
	usage, err := h.deps.StorageUsage.Usage()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func (h handlers) loadSetupStatus(ctx context.Context) (model.SetupStatus, error) {
	var status model.SetupStatus
	adminCount, err := h.deps.Users.Count(ctx)
	if err != nil {
		return model.SetupStatus{}, err
	}
	keyCount, err := h.deps.APIKeys.CountEnabled(ctx)
	if err != nil {
		return model.SetupStatus{}, err
	}
	apiKeyDoneRaw, apiKeyDone, err := h.deps.Settings.Get(ctx, setupAPIKeyDoneKey)
	if err != nil {
		return model.SetupStatus{}, err
	}
	listenRulesDoneRaw, listenRulesDone, err := h.deps.Settings.Get(ctx, setupListenRulesDoneKey)
	if err != nil {
		return model.SetupStatus{}, err
	}
	completeRaw, ok, err := h.deps.Settings.Get(ctx, "setup.complete")
	if err != nil {
		return model.SetupStatus{}, err
	}
	telegramAPI, err := h.deps.Settings.LoadTelegramAPI(ctx)
	if err != nil {
		return model.SetupStatus{}, err
	}
	status.AdminConfigured = adminCount > 0
	status.APIKeyConfigured = keyCount > 0
	status.APIKeyStepComplete = status.APIKeyConfigured || (apiKeyDone && apiKeyDoneRaw == `true`)
	status.TelegramConfigured = repository.RedactTelegramAPI(telegramAPI).Configured ||
		(h.deps.RuntimeConfig.Telegram.APIID > 0 && h.deps.RuntimeConfig.Telegram.APIHash != "")
	if h.deps.Accounts != nil {
		accounts, err := h.deps.Accounts.FindAll(ctx)
		if err != nil {
			return model.SetupStatus{}, err
		}
		status.TelegramLoginComplete = len(accounts) > 0
	}
	status.ListenRulesConfigured = listenRulesDone && listenRulesDoneRaw == `true`
	status.Complete = ok && completeRaw == `true`
	status.CurrentStep = setupCurrentStep(status)
	return status, nil
}

func setupCurrentStep(status model.SetupStatus) string {
	switch {
	case status.Complete:
		return "complete"
	case !status.AdminConfigured:
		return "admin"
	case !status.APIKeyStepComplete:
		return "api_key"
	case !status.TelegramConfigured:
		return "telegram_api"
	case !status.TelegramLoginComplete:
		return "telegram_login"
	case !status.ListenRulesConfigured:
		return "listen_rules"
	default:
		return "channel_selection"
	}
}

func (h handlers) status(c *gin.Context) {
	counts, err := h.deps.Status.Counts(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"service":        "ok",
		"accounts":       counts.Accounts,
		"channels":       counts.Channels,
		"messages":       counts.Messages,
		"links":          counts.Links,
		"account_states": counts.AccountStates,
	})
}

func (h handlers) tasks(c *gin.Context) {
	if h.deps.TaskRepository == nil {
		errorText(c, http.StatusServiceUnavailable, "tasks are unavailable")
		return
	}
	limit, ok := queryNonNegativeInt(c, "limit")
	if !ok {
		return
	}
	offset, ok := queryNonNegativeInt(c, "offset")
	if !ok {
		return
	}
	items, err := h.deps.TaskRepository.List(c.Request.Context(), taskpkg.ListFilter{
		Status: c.Query("status"),
		Type:   c.Query("type"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) task(c *gin.Context) {
	if h.deps.TaskRepository == nil {
		errorText(c, http.StatusServiceUnavailable, "tasks are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.deps.TaskRepository.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) retryTask(c *gin.Context) {
	h.updateTask(c, func(ctx context.Context, id int64) error {
		return h.deps.Tasks.Retry(ctx, id)
	})
}

func (h handlers) cancelTask(c *gin.Context) {
	h.updateTask(c, func(ctx context.Context, id int64) error {
		return h.deps.Tasks.Cancel(ctx, id)
	})
}

func (h handlers) pauseTask(c *gin.Context) {
	h.updateTask(c, func(ctx context.Context, id int64) error {
		return h.deps.Tasks.Pause(ctx, id)
	})
}

func (h handlers) resumeTask(c *gin.Context) {
	h.updateTask(c, func(ctx context.Context, id int64) error {
		return h.deps.Tasks.Resume(ctx, id)
	})
}

func (h handlers) updateTask(c *gin.Context, fn func(context.Context, int64) error) {
	if h.deps.Tasks == nil || h.deps.TaskRepository == nil {
		errorText(c, http.StatusServiceUnavailable, "tasks are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := fn(c.Request.Context(), id); err != nil {
		if errors.Is(err, taskpkg.ErrInvalidTransition) {
			errorJSON(c, http.StatusConflict, err)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			errorJSON(c, http.StatusNotFound, err)
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	item, err := h.deps.TaskRepository.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	h.publishEvent(taskpkg.Event{Type: taskpkg.EventTaskUpdated, Payload: item})
	c.JSON(http.StatusOK, item)
}

func (h handlers) events(c *gin.Context) {
	if h.deps.Events == nil {
		errorText(c, http.StatusServiceUnavailable, "events are unavailable")
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	_, _ = c.Writer.WriteString(": connected\n\n")
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	events, unsubscribe := h.deps.Events.Subscribe(c.Request.Context())
	defer unsubscribe()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, data)
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func (h handlers) publishEvent(event taskpkg.Event) {
	if h.deps.Events != nil {
		h.deps.Events.Publish(event)
	}
}

func (h handlers) sendCode(c *gin.Context) {
	var req struct {
		Phone string `json:"phone"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Phone == "" {
		errorText(c, http.StatusBadRequest, "phone is required")
		return
	}
	accountID, err := h.deps.Accounts.Save(c.Request.Context(), model.Account{
		Phone:  req.Phone,
		Status: model.AccountStatusLoginRequired,
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	sessionPath := h.sessionPath(accountID)
	sent, err := h.deps.Telegram.SendCode(c.Request.Context(), req.Phone, sessionPath)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	h.deps.CodeStore.Save(req.Phone, sent.PhoneCodeHash)
	c.JSON(http.StatusOK, gin.H{"status": model.AccountStatusLoginRequired})
}

func (h handlers) signIn(c *gin.Context) {
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Phone == "" || req.Code == "" {
		errorText(c, http.StatusBadRequest, "phone and code are required")
		return
	}
	account, err := h.deps.Accounts.FindByPhone(c.Request.Context(), req.Phone)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		errorJSON(c, status, err)
		return
	}
	hash, ok := h.deps.CodeStore.Take(req.Phone)
	if !ok {
		errorText(c, http.StatusBadRequest, "login code hash is missing; call send-code first")
		return
	}
	profile, err := h.deps.Telegram.SignIn(c.Request.Context(), req.Phone, req.Code, hash, h.sessionPath(account.ID))
	if err != nil {
		if errors.Is(err, telegram.ErrPasswordRequired) {
			c.JSON(http.StatusAccepted, gin.H{"status": model.AccountStatusLoginRequired, "password_required": true})
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	h.updateAccountProfile(c, account, profile)
}

func (h handlers) password(c *gin.Context) {
	var req struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Phone == "" || req.Password == "" {
		errorText(c, http.StatusBadRequest, "phone and password are required")
		return
	}
	account, err := h.deps.Accounts.FindByPhone(c.Request.Context(), req.Phone)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	profile, err := h.deps.Telegram.Password(c.Request.Context(), req.Password, h.sessionPath(account.ID))
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	h.updateAccountProfile(c, account, profile)
}

func (h handlers) accounts(c *gin.Context) {
	items, err := h.deps.Accounts.FindAll(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) logoutAccount(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.deps.Accounts.FindByID(c.Request.Context(), id); err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	if h.deps.AccountRuntime != nil {
		if err := h.deps.AccountRuntime.StopAccount(c.Request.Context(), id); err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			return
		}
	}
	if h.deps.Sessions != nil {
		if err := h.deps.Sessions.RemoveForAccount(id); err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			return
		}
	}
	if err := h.deps.Accounts.UpdateStatus(c.Request.Context(), id, model.AccountStatusLoginRequired); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	account, err := h.deps.Accounts.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h handlers) deleteAccount(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if h.deps.AccountRuntime != nil {
		if err := h.deps.AccountRuntime.StopAccount(c.Request.Context(), id); err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			return
		}
	}
	if h.deps.Sessions != nil {
		if err := h.deps.Sessions.RemoveForAccount(id); err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			return
		}
	}
	if err := h.deps.Accounts.Delete(c.Request.Context(), id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h handlers) channels(c *gin.Context) {
	accountID, ok := queryPositiveInt64(c, "account_id")
	if !ok {
		return
	}
	var (
		items []model.Channel
		err   error
	)
	if accountID > 0 {
		items, err = h.deps.Channels.FindByAccountID(c.Request.Context(), accountID)
	} else {
		items, err = h.deps.Channels.FindAll(c.Request.Context())
	}
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) channel(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.deps.Channels.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) updateChannelControl(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req model.ChannelControl
	if !bindJSON(c, &req) {
		return
	}
	profile, err := channelpkg.ParseProfile(req.SyncProfile)
	if err != nil {
		errorText(c, http.StatusBadRequest, err.Error())
		return
	}
	req.SyncProfile = profile
	if profile == channelpkg.SyncProfileDeep || profile == channelpkg.SyncProfileFull {
		if !h.checkStorageQuota(c) {
			return
		}
	}
	if err := h.deps.Channels.UpdateControl(c.Request.Context(), id, req); err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	item, err := h.deps.Channels.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) analyzeChannel(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.deps.Channels.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	var watchRule *model.WatchRule
	if h.deps.WatchRules != nil {
		rule, err := h.deps.WatchRules.FindByChannelID(c.Request.Context(), id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			errorJSON(c, http.StatusInternalServerError, err)
			return
		}
		if err == nil {
			watchRule = &rule
		}
	}
	c.JSON(http.StatusOK, model.ChannelAnalysis{
		Channel: item,
		Control: model.ChannelControl{
			HistorySyncEnabled:  item.HistorySyncEnabled,
			SyncProfile:         item.SyncProfile,
			ListenEnabled:       item.ListenEnabled,
			RemoteSearchAllowed: item.RemoteSearchAllowed,
		},
		WatchRule:     watchRule,
		IndexedCounts: model.ChannelIndexedCounts{},
	})
}

func (h handlers) createRemoteSearchTask(c *gin.Context) {
	var req struct {
		ChannelID int64  `json:"channel_id"`
		Query     string `json:"query"`
	}
	if !bindJSON(c, &req) {
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		errorText(c, http.StatusBadRequest, "query is required")
		return
	}
	if req.ChannelID <= 0 {
		errorText(c, http.StatusBadRequest, "channel_id must be a positive integer")
		return
	}
	if h.deps.RemoteSearch == nil {
		errorText(c, http.StatusServiceUnavailable, "remote search is unavailable")
		return
	}
	if h.deps.RemoteSearchExec != nil {
		task, err := h.deps.RemoteSearchExec.Search(c.Request.Context(), req.ChannelID, query, 50)
		if err != nil {
			h.remoteSearchError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, task)
		return
	}
	item, err := h.deps.Channels.FindByID(c.Request.Context(), req.ChannelID)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	if !item.RemoteSearchAllowed {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":    "remote_search_not_allowed",
			"message": "remote search is not allowed for this channel",
		}})
		return
	}
	if item.LastMessageID > 0 || item.LastSyncTime != nil {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":    "remote_search_requires_unsynced_channel",
			"message": "remote search requires an unsynced channel",
		}})
		return
	}
	task := model.RemoteSearchTask{
		AccountID: item.AccountID,
		ChannelID: item.ID,
		Query:     query,
		Status:    model.RemoteSearchStatusQueued,
		Source:    "remote",
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	id, err := h.deps.RemoteSearch.Create(c.Request.Context(), task)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	task, err = h.deps.RemoteSearch.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusAccepted, task)
}

func (h handlers) getRemoteSearchTask(c *gin.Context) {
	id, ok := pathPositiveInt64(c, "task_id")
	if !ok {
		return
	}
	if h.deps.RemoteSearchExec == nil {
		errorText(c, http.StatusServiceUnavailable, "remote search execution is unavailable")
		return
	}
	result, err := h.deps.RemoteSearchExec.Results(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) remoteSearchError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, searchsvc.ErrEmptyQuery):
		errorText(c, http.StatusBadRequest, "query is required")
	case errors.Is(err, searchsvc.ErrRemoteSearchNotAllowed):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":    "remote_search_not_allowed",
			"message": "remote search is not allowed for this channel",
		}})
	case errors.Is(err, searchsvc.ErrRemoteSearchRequiresUnsynced):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":    "remote_search_requires_unsynced_channel",
			"message": "remote search requires an unsynced channel",
		}})
	default:
		errorJSON(c, http.StatusInternalServerError, err)
	}
}

func (h handlers) checkStorageQuota(c *gin.Context) bool {
	if h.deps.StorageUsage == nil {
		return true
	}
	usage, err := h.deps.StorageUsage.Usage()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return false
	}
	if usage.MaxDBBytes > 0 && usage.DBBytes >= usage.MaxDBBytes {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":    "storage_quota_exceeded",
			"message": "database storage quota exceeded",
		}})
		return false
	}
	return true
}

func (h handlers) syncChannel(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if h.deps.SyncQueue != nil {
		jobCtx := context.WithoutCancel(c.Request.Context())
		job := h.deps.SyncQueue.Enqueue(jobCtx, "channel-sync", func(ctx context.Context) error {
			_, err := h.deps.History.SyncChannel(ctx, id)
			return err
		})
		c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "status": job.Status})
		return
	}
	result, err := h.deps.History.SyncChannel(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (h handlers) syncChannels(c *gin.Context) {
	var req struct {
		ChannelIDs []int64 `json:"channel_ids"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if len(req.ChannelIDs) == 0 {
		errorText(c, http.StatusBadRequest, "channel_ids is required")
		return
	}
	for _, id := range req.ChannelIDs {
		if id <= 0 {
			errorText(c, http.StatusBadRequest, "channel_ids must contain positive integers")
			return
		}
	}
	if h.deps.SyncQueue != nil {
		channelIDs := append([]int64(nil), req.ChannelIDs...)
		jobCtx := context.WithoutCancel(c.Request.Context())
		job := h.deps.SyncQueue.Enqueue(jobCtx, "channels-sync", func(ctx context.Context) error {
			result := h.deps.History.SyncMany(ctx, channelIDs)
			if len(result.Failures) > 0 {
				return fmt.Errorf("sync failures: %v", result.Failures)
			}
			return nil
		})
		c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "status": job.Status})
		return
	}
	result := h.deps.History.SyncMany(c.Request.Context(), req.ChannelIDs)
	c.JSON(http.StatusAccepted, result)
}

func (h handlers) checkChannelWebAccess(c *gin.Context) {
	var req struct {
		ChannelIDs []int64 `json:"channel_ids"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if len(req.ChannelIDs) == 0 {
		errorText(c, http.StatusBadRequest, "channel_ids is required")
		return
	}
	for _, id := range req.ChannelIDs {
		if id <= 0 {
			errorText(c, http.StatusBadRequest, "channel_ids must contain positive integers")
			return
		}
	}
	if h.deps.ChannelWebAccess == nil {
		errorText(c, http.StatusServiceUnavailable, "channel web access checker is unavailable")
		return
	}
	items, err := h.deps.ChannelWebAccess.CheckMany(c.Request.Context(), req.ChannelIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorJSON(c, http.StatusNotFound, err)
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) syncAccountChannels(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	account, err := h.deps.Accounts.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	if h.deps.SyncQueue != nil {
		jobCtx := context.WithoutCancel(c.Request.Context())
		job := h.deps.SyncQueue.Enqueue(jobCtx, "account-channels-sync", func(ctx context.Context) error {
			_, err := h.deps.ChannelSync.SyncAccount(ctx, account)
			return err
		})
		c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "status": job.Status})
		return
	}
	items, err := h.deps.ChannelSync.SyncAccount(c.Request.Context(), account)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"items": items})
}

type watchRulePayload struct {
	ChannelID    int64           `json:"channel_id"`
	Enabled      *bool           `json:"enabled"`
	Includes     json.RawMessage `json:"includes"`
	Excludes     json.RawMessage `json:"excludes"`
	MessageTypes json.RawMessage `json:"message_types"`
	LinkTypes    json.RawMessage `json:"link_types"`
}

func (h handlers) watchRules(c *gin.Context) {
	items, err := h.deps.WatchRules.FindAll(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) watchRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.deps.WatchRules.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) createWatchRule(c *gin.Context) {
	rule, ok := h.readWatchRuleRequest(c, true)
	if !ok {
		return
	}
	id, err := h.deps.WatchRules.Create(c.Request.Context(), rule)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateWatchRule) {
			errorText(c, http.StatusConflict, "watch rule already exists for channel")
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	item, err := h.deps.WatchRules.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h handlers) updateWatchRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	rule, ok := h.readWatchRuleRequest(c, false)
	if !ok {
		return
	}
	rule.ID = id
	if err := h.deps.WatchRules.Update(c.Request.Context(), rule); err != nil {
		if errors.Is(err, repository.ErrDuplicateWatchRule) {
			errorText(c, http.StatusConflict, "watch rule already exists for channel")
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			errorJSON(c, http.StatusNotFound, err)
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	item, err := h.deps.WatchRules.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) deleteWatchRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.deps.WatchRules.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			errorJSON(c, http.StatusNotFound, err)
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h handlers) readWatchRuleRequest(c *gin.Context, create bool) (model.WatchRule, bool) {
	var payload watchRulePayload
	if !bindJSON(c, &payload) {
		return model.WatchRule{}, false
	}
	if payload.ChannelID <= 0 {
		errorText(c, http.StatusBadRequest, "channel_id must be a positive integer")
		return model.WatchRule{}, false
	}
	if _, err := h.deps.Channels.FindByID(c.Request.Context(), payload.ChannelID); err != nil {
		errorText(c, http.StatusBadRequest, "channel_id must reference an existing channel")
		return model.WatchRule{}, false
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	} else if !create {
		errorText(c, http.StatusBadRequest, "enabled is required")
		return model.WatchRule{}, false
	}
	includes, ok := decodeStringArray(c, payload.Includes, "includes")
	if !ok {
		return model.WatchRule{}, false
	}
	excludes, ok := decodeStringArray(c, payload.Excludes, "excludes")
	if !ok {
		return model.WatchRule{}, false
	}
	messageTypes, ok := decodeStringArray(c, payload.MessageTypes, "message_types")
	if !ok {
		return model.WatchRule{}, false
	}
	linkTypes, ok := decodeStringArray(c, payload.LinkTypes, "link_types")
	if !ok {
		return model.WatchRule{}, false
	}
	return model.WatchRule{
		ChannelID:    payload.ChannelID,
		Enabled:      enabled,
		Includes:     includes,
		Excludes:     excludes,
		MessageTypes: messageTypes,
		LinkTypes:    linkTypes,
	}, true
}

func decodeStringArray(c *gin.Context, raw json.RawMessage, field string) ([]string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		errorText(c, http.StatusBadRequest, field+" must be an array of strings")
		return nil, false
	}
	return out, true
}

func normalizeSetupStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (h handlers) search(c *gin.Context) {
	accountID, channelID, limit, offset, ok := readFilters(c)
	if !ok {
		return
	}
	dateFrom, dateTo, ok := parseDateRange(c)
	if !ok {
		return
	}
	beforeDate, beforeID, ok := parseCursor(c)
	if !ok {
		return
	}
	items, err := h.deps.Search.Search(c.Request.Context(), searchsvc.Params{
		Query:      c.Query("q"),
		AccountID:  accountID,
		ChannelID:  channelID,
		LinkType:   c.Query("link_type"),
		Sort:       c.Query("sort"),
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		BeforeDate: beforeDate,
		BeforeID:   beforeID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, searchsvc.ErrEmptyQuery) {
			status = http.StatusBadRequest
		}
		errorJSON(c, status, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) searchGlobal(c *gin.Context) {
	query, ok := h.readSearchQuery(c)
	if !ok {
		return
	}
	result, err := h.deps.Search.Global(c.Request.Context(), query)
	if err != nil {
		h.searchError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) searchMessages(c *gin.Context) {
	query, ok := h.readSearchQuery(c)
	if !ok {
		return
	}
	result, err := h.deps.Search.Messages(c.Request.Context(), query)
	if err != nil {
		h.searchError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) searchLinks(c *gin.Context) {
	query, ok := h.readSearchQuery(c)
	if !ok {
		return
	}
	result, err := h.deps.Search.ScopedLinks(c.Request.Context(), query)
	if err != nil {
		h.searchError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) searchFiles(c *gin.Context) {
	query, ok := h.readSearchQuery(c)
	if !ok {
		return
	}
	result, err := h.deps.Search.Files(c.Request.Context(), query)
	if err != nil {
		h.searchError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) searchChannels(c *gin.Context) {
	query, ok := h.readSearchQuery(c)
	if !ok {
		return
	}
	result, err := h.deps.Search.Channels(c.Request.Context(), query)
	if err != nil {
		h.searchError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) readSearchQuery(c *gin.Context) (searchsvc.SearchQuery, bool) {
	accountID, channelID, limit, offset, ok := readFilters(c)
	if !ok {
		return searchsvc.SearchQuery{}, false
	}
	dateFrom, dateTo, ok := parseDateRange(c)
	if !ok {
		return searchsvc.SearchQuery{}, false
	}
	return searchsvc.SearchQuery{
		Query:       c.Query("q"),
		AccountID:   accountID,
		ChannelID:   channelID,
		MessageType: c.Query("message_type"),
		LinkType:    firstQuery(c, "link_type", "type"),
		FileType:    firstQuery(c, "file_type", "category"),
		Sort:        c.Query("sort"),
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		Limit:       limit,
		Offset:      offset,
	}, true
}

func (h handlers) searchError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, searchsvc.ErrEmptyQuery) {
		status = http.StatusBadRequest
	}
	errorJSON(c, status, err)
}

func firstQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := c.Query(key); value != "" {
			return value
		}
	}
	return ""
}

func (h handlers) latest(c *gin.Context) {
	accountID, channelID, limit, _, ok := readFilters(c)
	if !ok {
		return
	}
	beforeDate, beforeID, ok := parseCursor(c)
	if !ok {
		return
	}
	items, err := h.deps.Search.Latest(c.Request.Context(), searchsvc.LatestParams{
		AccountID:  accountID,
		ChannelID:  channelID,
		BeforeDate: beforeDate,
		BeforeID:   beforeID,
		Limit:      limit,
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": latestMessageItems(items)})
}

type latestMessageItem struct {
	ID                int64        `json:"id"`
	ChannelID         int64        `json:"channel_id"`
	TelegramMessageID int64        `json:"telegram_message_id"`
	SenderID          int64        `json:"sender_id"`
	Text              string       `json:"text"`
	RawJSON           string       `json:"raw_json"`
	Date              time.Time    `json:"date"`
	EditDate          *time.Time   `json:"edit_date,omitempty"`
	Deleted           bool         `json:"deleted"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	ChannelTitle      string       `json:"channel_title"`
	ChannelUsername   string       `json:"channel_username"`
	Links             []model.Link `json:"links"`
}

func latestMessageItems(items []model.SearchResult) []latestMessageItem {
	out := make([]latestMessageItem, len(items))
	for i, item := range items {
		links := item.Links
		if links == nil {
			links = []model.Link{}
		}
		out[i] = latestMessageItem{
			ID:                item.ID,
			ChannelID:         item.ChannelID,
			TelegramMessageID: item.TelegramMessageID,
			SenderID:          item.SenderID,
			Text:              item.Text,
			RawJSON:           item.RawJSON,
			Date:              item.Date,
			EditDate:          item.EditDate,
			Deleted:           item.Deleted,
			CreatedAt:         item.CreatedAt,
			UpdatedAt:         item.UpdatedAt,
			ChannelTitle:      item.ChannelTitle,
			ChannelUsername:   item.ChannelUsername,
			Links:             links,
		}
	}
	return out
}

func (h handlers) links(c *gin.Context) {
	accountID, channelID, limit, offset, ok := readFilters(c)
	if !ok {
		return
	}
	dateFrom, dateTo, ok := parseDateRange(c)
	if !ok {
		return
	}
	items, err := h.deps.Search.Links(c.Request.Context(), searchsvc.LinkParams{
		Type:      c.Query("type"),
		AccountID: accountID,
		ChannelID: channelID,
		Keyword:   c.Query("keyword"),
		Sort:      c.Query("sort"),
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) mergedLinks(c *gin.Context) {
	accountID, channelID, limit, offset, ok := readFilters(c)
	if !ok {
		return
	}
	dateFrom, dateTo, ok := parseDateRange(c)
	if !ok {
		return
	}
	keyword := c.Query("q")
	if keyword == "" {
		keyword = c.Query("keyword")
	}
	result, err := h.deps.Search.MergedLinks(c.Request.Context(), searchsvc.LinkParams{
		Type:      c.Query("type"),
		AccountID: accountID,
		ChannelID: channelID,
		Keyword:   keyword,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) resources(c *gin.Context) {
	if h.deps.Resources == nil {
		errorText(c, http.StatusServiceUnavailable, "resources are unavailable")
		return
	}
	query, ok := readResourceQuery(c)
	if !ok {
		return
	}
	result, err := h.deps.Resources.List(c.Request.Context(), query)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h handlers) resourcesGrouped(c *gin.Context) {
	if h.deps.Resources == nil {
		errorText(c, http.StatusServiceUnavailable, "resources are unavailable")
		return
	}
	query, ok := readResourceQuery(c)
	if !ok {
		return
	}
	query.Limit = 200
	result, err := h.deps.Resources.List(c.Request.Context(), query)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"grouped": result.Grouped})
}

func (h handlers) resource(c *gin.Context) {
	if h.deps.Resources == nil {
		errorText(c, http.StatusServiceUnavailable, "resources are unavailable")
		return
	}
	id := c.Param("id")
	result, err := h.deps.Resources.List(c.Request.Context(), resource.Query{Limit: 200})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	for _, item := range result.Items {
		if item.ID == id {
			c.JSON(http.StatusOK, item)
			return
		}
	}
	errorText(c, http.StatusNotFound, "resource not found")
}

func readResourceQuery(c *gin.Context) (resource.Query, bool) {
	accountID, channelID, limit, offset, ok := readFilters(c)
	if !ok {
		return resource.Query{}, false
	}
	keyword := c.Query("q")
	if keyword == "" {
		keyword = c.Query("keyword")
	}
	return resource.Query{
		Keyword:   keyword,
		Type:      c.Query("type"),
		Category:  c.Query("category"),
		AccountID: accountID,
		ChannelID: channelID,
		Extension: c.Query("extension"),
		Sort:      c.Query("sort"),
		Limit:     limit,
		Offset:    offset,
	}, true
}

func (h handlers) maintenanceSQLite(c *gin.Context) {
	if h.deps.Maintenance == nil {
		errorText(c, http.StatusServiceUnavailable, "maintenance repository is unavailable")
		return
	}
	ops, err := h.deps.Maintenance.OptimizeSQLite(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"operations": ops})
}

func (h handlers) maintenanceBackup(c *gin.Context) {
	if h.deps.BackupDB == nil || h.deps.BackupDir == "" {
		errorText(c, http.StatusServiceUnavailable, "backup is unavailable")
		return
	}
	path, err := backup.SQLite(c.Request.Context(), h.deps.BackupDB, h.deps.BackupDir)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path})
}

func (h handlers) updateAccountProfile(c *gin.Context, account model.Account, profile telegram.Profile) {
	account.TelegramUserID = profile.TelegramUserID
	account.FirstName = profile.FirstName
	account.LastName = profile.LastName
	account.Username = profile.Username
	account.Status = model.AccountStatusOnline
	account.SessionPath = h.sessionPath(account.ID)
	now := time.Now().UTC()
	account.LastOnlineAt = &now
	account.LastError = ""
	if err := h.deps.Accounts.Update(c.Request.Context(), account); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	metadataSync := gin.H{"status": "skipped", "channel_count": 0}
	if h.deps.ChannelSync != nil {
		items, err := h.deps.ChannelSync.SyncAccount(c.Request.Context(), account)
		if err != nil {
			account.LastError = err.Error()
			if updateErr := h.deps.Accounts.Update(c.Request.Context(), account); updateErr != nil {
				errorJSON(c, http.StatusInternalServerError, updateErr)
				return
			}
			metadataSync = gin.H{"status": "failed", "channel_count": 0, "error": err.Error()}
		} else {
			metadataSync = gin.H{"status": "succeeded", "channel_count": len(items)}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"status":        model.AccountStatusOnline,
		"account":       account,
		"metadata_sync": metadataSync,
	})
}

func (h handlers) sessionPath(accountID int64) string {
	if h.deps.Sessions == nil {
		return ""
	}
	return h.deps.Sessions.PathForAccount(accountID)
}

func bindJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		errorJSON(c, http.StatusBadRequest, err)
		return false
	}
	return true
}

func pathID(c *gin.Context) (int64, bool) {
	return pathPositiveInt64(c, "id")
}

func pathPositiveInt64(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id <= 0 {
		errorText(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func queryPositiveInt64(c *gin.Context, key string) (int64, bool) {
	value := c.Query(key)
	if value == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		errorText(c, http.StatusBadRequest, key+" must be a positive integer")
		return 0, false
	}
	return n, true
}

func queryNonNegativeInt(c *gin.Context, key string) (int, bool) {
	value := c.Query(key)
	if value == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		errorText(c, http.StatusBadRequest, key+" must be a non-negative integer")
		return 0, false
	}
	if int64(int(n)) != n {
		errorText(c, http.StatusBadRequest, key+" is too large")
		return 0, false
	}
	return int(n), true
}

func readFilters(c *gin.Context) (accountID int64, channelID int64, limit int, offset int, ok bool) {
	accountID, ok = queryPositiveInt64(c, "account_id")
	if !ok {
		return 0, 0, 0, 0, false
	}
	channelID, ok = queryPositiveInt64(c, "channel_id")
	if !ok {
		return 0, 0, 0, 0, false
	}
	limit, ok = queryNonNegativeInt(c, "limit")
	if !ok {
		return 0, 0, 0, 0, false
	}
	offset, ok = queryNonNegativeInt(c, "offset")
	if !ok {
		return 0, 0, 0, 0, false
	}
	return accountID, channelID, limit, offset, true
}

func parseDateRange(c *gin.Context) (*time.Time, *time.Time, bool) {
	from, ok := parseDateQuery(c, "date_from", false)
	if !ok {
		return nil, nil, false
	}
	to, ok := parseDateQuery(c, "date_to", true)
	if !ok {
		return nil, nil, false
	}
	return from, to, true
}

func parseDateQuery(c *gin.Context, key string, end bool) (*time.Time, bool) {
	value := c.Query(key)
	if value == "" {
		return nil, true
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		if end {
			t = t.Add(time.Nanosecond)
		}
		return &t, true
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		if end {
			t = t.AddDate(0, 0, 1)
		}
		return &t, true
	}
	errorText(c, http.StatusBadRequest, key+" must be YYYY-MM-DD or RFC3339")
	return nil, false
}

func parseCursor(c *gin.Context) (*time.Time, int64, bool) {
	beforeDateRaw := c.Query("before_date")
	beforeIDRaw := c.Query("before_id")
	if beforeDateRaw == "" && beforeIDRaw == "" {
		return nil, 0, true
	}
	if beforeDateRaw == "" || beforeIDRaw == "" {
		errorText(c, http.StatusBadRequest, "before_date and before_id must be provided together")
		return nil, 0, false
	}
	beforeDate, ok := parseDateQuery(c, "before_date", false)
	if !ok {
		return nil, 0, false
	}
	beforeID, err := strconv.ParseInt(beforeIDRaw, 10, 64)
	if err != nil || beforeID <= 0 {
		errorText(c, http.StatusBadRequest, "before_id must be a positive integer")
		return nil, 0, false
	}
	return beforeDate, beforeID, true
}

func errorJSON(c *gin.Context, status int, err error) {
	errorText(c, status, err.Error())
}

func errorText(c *gin.Context, status int, msg string) {
	code := "internal_error"
	if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
		code = "bad_request"
	}
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": msg}})
}
