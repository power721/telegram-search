package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tg-search/internal/model"
	"tg-search/internal/storage"
)

func (h handlers) serveChannelAvatar(c *gin.Context) {
	if h.deps.Telegram == nil {
		errorText(c, http.StatusServiceUnavailable, "telegram client is unavailable")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		errorText(c, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	channel, err := h.deps.Channels.FindByID(c.Request.Context(), id)
	if err != nil {
		errorText(c, http.StatusNotFound, "channel not found")
		return
	}
	if channel.PhotoID <= 0 {
		errorText(c, http.StatusNotFound, "channel has no avatar")
		return
	}

	cacheKey := channelAvatarCacheKey(channel)
	if entry, hit := h.avatarCacheGet(c.Request.Context(), cacheKey); hit {
		serveAvatarData(c, http.DetectContentType(entry.Data), entry.Data)
		return
	}

	account, err := h.deps.Accounts.FindByID(c.Request.Context(), channel.AccountID)
	if err != nil {
		errorText(c, http.StatusNotFound, "account not found")
		return
	}
	session := h.accountSession(account)

	var imageData []byte
	var imageMIME string
	downloadErr := h.runMediaDownload(c.Request.Context(), func() error {
		if entry, hit := h.avatarCacheGet(c.Request.Context(), cacheKey); hit {
			imageData = entry.Data
			imageMIME = http.DetectContentType(entry.Data)
			return nil
		}
		img, err := h.deps.Telegram.DownloadChannelAvatar(
			c.Request.Context(),
			session,
			channel.TelegramChannelID,
			channel.AccessHash,
			channel.PhotoID,
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

func serveAvatarData(c *gin.Context, mime string, data []byte) {
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	c.Header("Content-Type", mime)
	c.Header("Content-Length", strconv.Itoa(len(data)))
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.Data(http.StatusOK, mime, data)
}

func channelAvatarCacheKey(channel model.Channel) string {
	return fmt.Sprintf("ch-avatar:%d:%d", channel.ID, channel.PhotoID)
}

func (h handlers) avatarCacheGet(ctx context.Context, key string) (storage.MediaCacheEntry, bool) {
	if h.deps.AvatarCache == nil {
		return storage.MediaCacheEntry{}, false
	}
	entry, hit, err := h.deps.AvatarCache.Get(ctx, key)
	if err != nil {
		h.deps.Logger.Warn("read avatar cache failed", zap.Error(err))
		return storage.MediaCacheEntry{}, false
	}
	return entry, hit
}

func (h handlers) avatarCacheSet(ctx context.Context, key string, data []byte) {
	if h.deps.AvatarCache == nil {
		return
	}
	if err := h.deps.AvatarCache.Set(ctx, key, data); err != nil {
		h.deps.Logger.Warn("write avatar cache failed", zap.Error(err))
	}
}
