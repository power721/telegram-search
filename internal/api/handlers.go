package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"tg-provider/internal/model"
	searchsvc "tg-provider/internal/search"
	"tg-provider/internal/telegram"
)

type handlers struct {
	deps Dependencies
}

func (h handlers) status(c *gin.Context) {
	counts, err := h.deps.Status.Counts(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"service":  "ok",
		"accounts": counts.Accounts,
		"channels": counts.Channels,
		"messages": counts.Messages,
		"links":    counts.Links,
	})
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

func (h handlers) deleteAccount(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.deps.Accounts.Delete(c.Request.Context(), id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h handlers) channels(c *gin.Context) {
	accountID := queryInt(c, "account_id")
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

func (h handlers) syncChannel(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	result, err := h.deps.History.SyncChannel(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (h handlers) search(c *gin.Context) {
	items, err := h.deps.Search.Search(c.Request.Context(), searchsvc.Params{
		Query:     c.Query("q"),
		AccountID: queryInt(c, "account_id"),
		ChannelID: queryInt(c, "channel_id"),
		LinkType:  c.Query("link_type"),
		Limit:     queryIntValue(c, "limit"),
		Offset:    queryIntValue(c, "offset"),
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

func (h handlers) latest(c *gin.Context) {
	items, err := h.deps.Search.Latest(c.Request.Context(), searchsvc.LatestParams{
		AccountID: queryInt(c, "account_id"),
		ChannelID: queryInt(c, "channel_id"),
		Limit:     queryIntValue(c, "limit"),
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) links(c *gin.Context) {
	items, err := h.deps.Search.Links(c.Request.Context(), searchsvc.LinkParams{
		Type:      c.Query("type"),
		AccountID: queryInt(c, "account_id"),
		ChannelID: queryInt(c, "channel_id"),
		Keyword:   c.Query("keyword"),
		Limit:     queryIntValue(c, "limit"),
		Offset:    queryIntValue(c, "offset"),
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) updateAccountProfile(c *gin.Context, account model.Account, profile telegram.Profile) {
	account.TelegramUserID = profile.TelegramUserID
	account.FirstName = profile.FirstName
	account.LastName = profile.LastName
	account.Username = profile.Username
	account.Status = model.AccountStatusOnline
	if err := h.deps.Accounts.Update(c.Request.Context(), account); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": model.AccountStatusOnline})
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
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		errorText(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func queryInt(c *gin.Context, key string) int64 {
	value := c.Query(key)
	if value == "" {
		return 0
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func queryIntValue(c *gin.Context, key string) int {
	return int(queryInt(c, key))
}

func errorJSON(c *gin.Context, status int, err error) {
	errorText(c, status, err.Error())
}

func errorText(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}
