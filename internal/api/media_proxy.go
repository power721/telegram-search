package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tg-search/internal/model"
	"tg-search/internal/telegram"
)

func (h handlers) requireMediaAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.hasAdminSession(c) {
			c.Next()
			return
		}
		if h.deps.APIKeyService == nil {
			errorText(c, http.StatusServiceUnavailable, "api key service is unavailable")
			c.Abort()
			return
		}
		if key := apiKeyFromRequestHeader(c.Request); key != "" {
			_, ok, err := h.deps.APIKeyService.Verify(c.Request.Context(), key)
			if err != nil {
				errorJSON(c, http.StatusInternalServerError, err)
				c.Abort()
				return
			}
			if !ok {
				errorText(c, http.StatusUnauthorized, "invalid api key")
				c.Abort()
				return
			}
			c.Next()
			return
		}
		exp := strings.TrimSpace(c.Query("exp"))
		sig := strings.TrimSpace(c.Query("sig"))
		if exp == "" || sig == "" {
			errorText(c, http.StatusUnauthorized, "media signature is required")
			c.Abort()
			return
		}
		ok, err := h.deps.APIKeyService.VerifyMediaSignature(c.Request.Context(), c.Request.Method, c.Request.URL.EscapedPath(), exp, sig, time.Now())
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			c.Abort()
			return
		}
		if !ok {
			errorText(c, http.StatusUnauthorized, "invalid media signature")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h handlers) serveTelegramVideo(c *gin.Context) {
	session, channel, msgID, ok := h.mediaRequestContext(c)
	if !ok {
		return
	}
	file, err := h.deps.Telegram.VideoFile(c.Request.Context(), session, channel, msgID)
	if err != nil {
		errorText(c, mediaErrorStatus(err), err.Error())
		return
	}
	mime := file.MIMEType
	if mime == "" {
		mime = "video/mp4"
	}
	start, end, partial, err := parseRange(c.GetHeader("Range"), file.Size)
	if err != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", file.Size))
		errorText(c, http.StatusRequestedRangeNotSatisfiable, "bad range")
		return
	}
	length := end - start + 1
	c.Header("Content-Type", mime)
	c.Header("Content-Length", strconv.FormatInt(length, 10))
	c.Header("Accept-Ranges", "bytes")
	if partial {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Size))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}
	if err := h.deps.Telegram.StreamVideoRange(c.Request.Context(), session, channel, msgID, file, start, length, c.Writer); err != nil {
		c.Error(err)
		return
	}
}

func (h handlers) serveTelegramImage(c *gin.Context) {
	session, channel, msgID, ok := h.mediaRequestContext(c)
	if !ok {
		return
	}
	image, err := h.deps.Telegram.DownloadMessageImage(c.Request.Context(), session, channel, msgID)
	if err != nil {
		errorText(c, mediaErrorStatus(err), err.Error())
		return
	}
	mime := image.MIMEType
	if mime == "" {
		mime = http.DetectContentType(image.Data)
	}
	c.Header("Content-Type", mime)
	c.Header("Content-Length", strconv.Itoa(len(image.Data)))
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, mime, image.Data)
}

func (h handlers) mediaRequestContext(c *gin.Context) (telegram.AccountSession, telegram.MediaChannelRef, int, bool) {
	if h.deps.Telegram == nil {
		errorText(c, http.StatusServiceUnavailable, "telegram client is unavailable")
		return telegram.AccountSession{}, telegram.MediaChannelRef{}, 0, false
	}
	msgID, err := strconv.Atoi(c.Param("msgid"))
	if err != nil || msgID <= 0 {
		errorText(c, http.StatusBadRequest, "msgid must be a positive integer")
		return telegram.AccountSession{}, telegram.MediaChannelRef{}, 0, false
	}
	channelParam := strings.TrimPrefix(strings.TrimSpace(c.Param("channel")), "@")
	if channelParam == "" {
		errorText(c, http.StatusBadRequest, "channel is required")
		return telegram.AccountSession{}, telegram.MediaChannelRef{}, 0, false
	}
	session, channel, err := h.resolveMediaSession(c.Request.Context(), channelParam)
	if err != nil {
		errorText(c, mediaErrorStatus(err), err.Error())
		return telegram.AccountSession{}, telegram.MediaChannelRef{}, 0, false
	}
	return session, channel, msgID, true
}

func (h handlers) resolveMediaSession(ctx context.Context, channelParam string) (telegram.AccountSession, telegram.MediaChannelRef, error) {
	if h.deps.Accounts == nil {
		return telegram.AccountSession{}, telegram.MediaChannelRef{}, fmt.Errorf("accounts are unavailable")
	}
	var matchedChannel model.Channel
	if h.deps.Channels != nil {
		channels, err := h.deps.Channels.FindAll(ctx)
		if err != nil {
			return telegram.AccountSession{}, telegram.MediaChannelRef{}, err
		}
		numericChannelID, _ := strconv.ParseInt(channelParam, 10, 64)
		for _, channel := range channels {
			if strings.EqualFold(strings.TrimPrefix(channel.Username, "@"), channelParam) || (numericChannelID > 0 && channel.ID == numericChannelID) {
				matchedChannel = channel
				break
			}
		}
	}
	if matchedChannel.ID > 0 {
		account, err := h.deps.Accounts.FindByID(ctx, matchedChannel.AccountID)
		if err != nil {
			return telegram.AccountSession{}, telegram.MediaChannelRef{}, err
		}
		return h.accountSession(account), telegram.MediaChannelRef{
			Username:          channelParam,
			TelegramChannelID: matchedChannel.TelegramChannelID,
			AccessHash:        matchedChannel.AccessHash,
			Type:              matchedChannel.Type,
		}, nil
	}
	accounts, err := h.deps.Accounts.FindAll(ctx)
	if err != nil {
		return telegram.AccountSession{}, telegram.MediaChannelRef{}, err
	}
	for _, account := range accounts {
		if account.Status == model.AccountStatusOnline || account.Status == model.AccountStatusSyncing || account.Status == model.AccountStatusReconnecting {
			return h.accountSession(account), telegram.MediaChannelRef{Username: channelParam}, nil
		}
	}
	return telegram.AccountSession{}, telegram.MediaChannelRef{}, sql.ErrNoRows
}

func (h handlers) accountSession(account model.Account) telegram.AccountSession {
	sessionPath := account.SessionPath
	if h.deps.Sessions != nil {
		sessionPath = h.deps.Sessions.PathForAccount(account.ID)
	}
	return telegram.AccountSession{
		AccountID:   account.ID,
		Phone:       account.Phone,
		SessionPath: sessionPath,
	}
}

func parseRange(h string, size int64) (start, end int64, partial bool, err error) {
	if size <= 0 {
		return 0, 0, false, fmt.Errorf("invalid size")
	}
	if h == "" {
		return 0, size - 1, false, nil
	}
	if !strings.HasPrefix(h, "bytes=") {
		return 0, 0, false, fmt.Errorf("invalid range")
	}
	value := strings.TrimPrefix(h, "bytes=")
	if strings.Contains(value, ",") {
		return 0, 0, false, fmt.Errorf("invalid range")
	}
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("invalid range")
	}
	if parts[0] == "" {
		suffix, parseErr := strconv.ParseInt(parts[1], 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, false, fmt.Errorf("invalid range")
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true, nil
	}
	start, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false, err
	}
	if parts[1] == "" {
		end = size - 1
	} else {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, false, err
		}
	}
	if start < 0 || end < start || end >= size {
		return 0, 0, false, fmt.Errorf("invalid range")
	}
	return start, end, true, nil
}

func mediaErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "bad range"), strings.Contains(msg, "invalid range"):
		return http.StatusRequestedRangeNotSatisfiable
	case strings.Contains(msg, "not found"), strings.Contains(msg, "no rows"), strings.Contains(msg, "has no"), strings.Contains(msg, "no usable photo size"):
		return http.StatusNotFound
	case strings.Contains(msg, "required"), strings.Contains(msg, "positive integer"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
