package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"tg-search/internal/model"
)

func (h handlers) serveAccountAvatar(c *gin.Context) {
	if h.deps.Telegram == nil {
		errorText(c, http.StatusServiceUnavailable, "telegram client is unavailable")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		errorText(c, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	account, err := h.deps.Accounts.FindByID(c.Request.Context(), id)
	if err != nil {
		errorText(c, http.StatusNotFound, "account not found")
		return
	}
	if account.PhotoID <= 0 {
		errorText(c, http.StatusNotFound, "account has no avatar")
		return
	}

	cacheKey := accountAvatarCacheKey(account)

	// Set ETag based on photo ID for efficient browser caching
	etag := fmt.Sprintf(`"acc-%d-%d"`, account.ID, account.PhotoID)
	c.Header("ETag", etag)

	// Check If-None-Match header for 304 Not Modified response
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	if entry, hit := h.avatarCacheGet(c.Request.Context(), cacheKey); hit {
		serveAvatarData(c, http.DetectContentType(entry.Data), entry.Data)
		return
	}

	session := h.accountSession(account)

	var imageData []byte
	var imageMIME string
	downloadErr := h.downloadAvatar(c.Request.Context(), session, cacheKey, func() error {
		if entry, hit := h.avatarCacheGet(c.Request.Context(), cacheKey); hit {
			imageData = entry.Data
			imageMIME = http.DetectContentType(entry.Data)
			return nil
		}
		img, err := h.deps.Telegram.DownloadUserAvatar(
			c.Request.Context(),
			session,
			account.TelegramUserID,
			account.PhotoID,
		)
		if err != nil {
			return err
		}
		imageData = img.Data
		imageMIME = img.MIMEType
		h.avatarCacheSet(c.Request.Context(), cacheKey, imageData)
		return nil
	})
	if downloadErr != nil {
		h.markMediaAccountAuthFailure(c.Request.Context(), session, downloadErr)
		errorText(c, mediaErrorStatus(downloadErr), downloadErr.Error())
		return
	}

	mime := imageMIME
	if mime == "" {
		mime = http.DetectContentType(imageData)
	}
	serveAvatarData(c, mime, imageData)
}

func accountAvatarCacheKey(account model.Account) string {
	return fmt.Sprintf("acc-avatar:%d:%d", account.ID, account.PhotoID)
}
