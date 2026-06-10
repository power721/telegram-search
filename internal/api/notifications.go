package api

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"tg-search/internal/model"
	"tg-search/internal/notification"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
)

type savedSearchRequest struct {
	Name           string                   `json:"name"`
	Keyword        string                   `json:"keyword"`
	Filters        model.SavedSearchFilters `json:"filters"`
	NotifyRSS      *bool                    `json:"notify_rss"`
	NotifyWebhook  *bool                    `json:"notify_webhook"`
	NotifyTelegram *bool                    `json:"notify_telegram"`
	Enabled        *bool                    `json:"enabled"`
}

type webhookRequest struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Secret  *string  `json:"secret"`
	Enabled *bool    `json:"enabled"`
}

func (h handlers) savedSearches(c *gin.Context) {
	if h.deps.SavedSearches == nil {
		errorText(c, http.StatusServiceUnavailable, "saved searches are unavailable")
		return
	}
	items, err := h.deps.SavedSearches.FindAll(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) savedSearch(c *gin.Context) {
	if h.deps.SavedSearches == nil {
		errorText(c, http.StatusServiceUnavailable, "saved searches are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.deps.SavedSearches.FindByID(c.Request.Context(), id)
	if err != nil {
		handleNotFound(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) createSavedSearch(c *gin.Context) {
	if h.deps.SavedSearches == nil {
		errorText(c, http.StatusServiceUnavailable, "saved searches are unavailable")
		return
	}
	item, ok := readSavedSearchRequest(c, 0)
	if !ok {
		return
	}
	id, err := h.deps.SavedSearches.Create(c.Request.Context(), item)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	created, err := h.deps.SavedSearches.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h handlers) updateSavedSearch(c *gin.Context) {
	if h.deps.SavedSearches == nil {
		errorText(c, http.StatusServiceUnavailable, "saved searches are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, ok := readSavedSearchRequest(c, id)
	if !ok {
		return
	}
	if err := h.deps.SavedSearches.Update(c.Request.Context(), item); err != nil {
		handleRepositoryWriteError(c, err)
		return
	}
	updated, err := h.deps.SavedSearches.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h handlers) deleteSavedSearch(c *gin.Context) {
	if h.deps.SavedSearches == nil {
		errorText(c, http.StatusServiceUnavailable, "saved searches are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.deps.SavedSearches.Delete(c.Request.Context(), id); err != nil {
		handleRepositoryWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h handlers) testSavedSearch(c *gin.Context) {
	if h.deps.SavedSearches == nil || h.deps.Resources == nil {
		errorText(c, http.StatusServiceUnavailable, "saved search test is unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	search, err := h.deps.SavedSearches.FindByID(c.Request.Context(), id)
	if err != nil {
		handleNotFound(c, err)
		return
	}
	result, err := h.deps.Resources.List(c.Request.Context(), resource.Query{
		Keyword:   search.Keyword,
		Type:      search.Filters.Type,
		Category:  search.Filters.Category,
		AccountID: search.Filters.AccountID,
		ChannelID: search.Filters.ChannelID,
		Limit:     50,
		MaxLimit:  50,
		Sort:      "date_desc",
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	matches := []resource.Item{}
	for _, item := range result.Items {
		if notification.SavedSearchMatchesResource(search, item) {
			matches = append(matches, item)
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": matches, "total": len(matches)})
}

func readSavedSearchRequest(c *gin.Context, id int64) (model.SavedSearch, bool) {
	var req savedSearchRequest
	if !bindJSON(c, &req) {
		return model.SavedSearch{}, false
	}
	name := strings.TrimSpace(req.Name)
	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		errorText(c, http.StatusBadRequest, "keyword is required")
		return model.SavedSearch{}, false
	}
	if name == "" {
		name = keyword
	}
	item := model.SavedSearch{
		ID:             id,
		Name:           name,
		Keyword:        keyword,
		Filters:        req.Filters,
		NotifyRSS:      boolValue(req.NotifyRSS, true),
		NotifyWebhook:  boolValue(req.NotifyWebhook, false),
		NotifyTelegram: boolValue(req.NotifyTelegram, false),
		Enabled:        boolValue(req.Enabled, true),
	}
	return item, true
}

func (h handlers) webhooks(c *gin.Context) {
	if h.deps.Webhooks == nil {
		errorText(c, http.StatusServiceUnavailable, "webhooks are unavailable")
		return
	}
	items, err := h.deps.Webhooks.FindAll(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) webhook(c *gin.Context) {
	if h.deps.Webhooks == nil {
		errorText(c, http.StatusServiceUnavailable, "webhooks are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.deps.Webhooks.FindByID(c.Request.Context(), id)
	if err != nil {
		handleNotFound(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) createWebhook(c *gin.Context) {
	if h.deps.Webhooks == nil {
		errorText(c, http.StatusServiceUnavailable, "webhooks are unavailable")
		return
	}
	item, ok := h.readWebhookRequest(c, 0, nil)
	if !ok {
		return
	}
	id, err := h.deps.Webhooks.Create(c.Request.Context(), item)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	created, err := h.deps.Webhooks.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h handlers) updateWebhook(c *gin.Context) {
	if h.deps.Webhooks == nil {
		errorText(c, http.StatusServiceUnavailable, "webhooks are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	existing, err := h.deps.Webhooks.FindByID(c.Request.Context(), id)
	if err != nil {
		handleNotFound(c, err)
		return
	}
	item, ok := h.readWebhookRequest(c, id, &existing)
	if !ok {
		return
	}
	if err := h.deps.Webhooks.Update(c.Request.Context(), item); err != nil {
		handleRepositoryWriteError(c, err)
		return
	}
	updated, err := h.deps.Webhooks.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h handlers) deleteWebhook(c *gin.Context) {
	if h.deps.Webhooks == nil {
		errorText(c, http.StatusServiceUnavailable, "webhooks are unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.deps.Webhooks.Delete(c.Request.Context(), id); err != nil {
		handleRepositoryWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h handlers) readWebhookRequest(c *gin.Context, id int64, existing *model.Webhook) (model.Webhook, bool) {
	var req webhookRequest
	if !bindJSON(c, &req) {
		return model.Webhook{}, false
	}
	name := strings.TrimSpace(req.Name)
	hookURL := strings.TrimSpace(req.URL)
	if name == "" {
		name = hookURL
	}
	if hookURL == "" {
		errorText(c, http.StatusBadRequest, "url is required")
		return model.Webhook{}, false
	}
	parsed, err := url.ParseRequestURI(hookURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		errorText(c, http.StatusBadRequest, "url must be an absolute http or https URL")
		return model.Webhook{}, false
	}
	if len(req.Events) == 0 {
		errorText(c, http.StatusBadRequest, "events is required")
		return model.Webhook{}, false
	}
	secret := ""
	if existing != nil {
		secret = existing.Secret
	}
	if req.Secret != nil {
		secret = *req.Secret
	}
	return model.Webhook{
		ID:      id,
		Name:    name,
		URL:     hookURL,
		Events:  req.Events,
		Secret:  secret,
		Enabled: boolValue(req.Enabled, true),
	}, true
}

func (h handlers) notificationDeliveries(c *gin.Context) {
	if h.deps.Deliveries == nil {
		errorText(c, http.StatusServiceUnavailable, "notification deliveries are unavailable")
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
	items, err := h.deps.Deliveries.List(c.Request.Context(), repository.NotificationDeliveryListParams{
		Status: strings.TrimSpace(c.Query("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func handleNotFound(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	errorJSON(c, http.StatusInternalServerError, err)
}

func handleRepositoryWriteError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	errorJSON(c, http.StatusInternalServerError, err)
}
