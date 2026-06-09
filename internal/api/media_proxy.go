package api

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tg-search/internal/model"
	"tg-search/internal/retry"
	"tg-search/internal/telegram"
)

type mediaRequest struct {
	session   telegram.AccountSession
	channel   telegram.MediaChannelRef
	messageID int
	file      model.FileResult
}

const maxOpenEndedVideoRangeLength int64 = 8 * 1024 * 1024

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
	media, ok := h.mediaRequestContext(c)
	if !ok {
		return
	}
	file, err := h.deps.Telegram.VideoFile(c.Request.Context(), media.session, media.channel, media.messageID)
	if err != nil {
		h.markMediaAccountAuthFailure(c.Request.Context(), media.session, err)
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
	setMediaMetadataHeaders(c, media.file, file.Size)
	if partial {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Size))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}
	if c.Request.Method == http.MethodHead {
		return
	}
	if err := h.deps.Telegram.StreamVideoRange(c.Request.Context(), media.session, media.channel, media.messageID, file, start, length, c.Writer); err != nil {
		c.Error(err)
		return
	}
}

func (h handlers) serveTelegramImage(c *gin.Context) {
	media, ok := h.mediaRequestContext(c)
	if !ok {
		return
	}
	image, err := h.deps.Telegram.DownloadMessageImage(c.Request.Context(), media.session, media.channel, media.messageID)
	if err != nil {
		h.markMediaAccountAuthFailure(c.Request.Context(), media.session, err)
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
	setMediaMetadataHeaders(c, media.file, int64(len(image.Data)))
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.Data(http.StatusOK, mime, image.Data)
}

func setMediaMetadataHeaders(c *gin.Context, file model.FileResult, size int64) {
	c.Header("ETag", fmt.Sprintf(`W/"tg-file-%d-%d-%d"`, file.TelegramFileID, size, file.UpdatedAt.UnixNano()))
	if !file.UpdatedAt.IsZero() {
		c.Header("Last-Modified", file.UpdatedAt.UTC().Format(http.TimeFormat))
	}
	if file.FileName != "" {
		c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": file.FileName}))
	}
	c.Header("Cache-Control", "public, max-age=86400")
}

func (h handlers) mediaRequestContext(c *gin.Context) (mediaRequest, bool) {
	if h.deps.Telegram == nil {
		errorText(c, http.StatusServiceUnavailable, "telegram client is unavailable")
		return mediaRequest{}, false
	}
	fileID, err := strconv.ParseInt(strings.TrimSpace(c.Param("fileid")), 10, 64)
	if err != nil || fileID <= 0 {
		errorText(c, http.StatusBadRequest, "fileid must be a positive integer")
		return mediaRequest{}, false
	}
	media, err := h.resolveMediaFileSession(c.Request.Context(), fileID)
	if err != nil {
		errorText(c, mediaErrorStatus(err), err.Error())
		return mediaRequest{}, false
	}
	return media, true
}

func (h handlers) resolveMediaFileSession(ctx context.Context, fileID int64) (mediaRequest, error) {
	if h.deps.Files == nil {
		return mediaRequest{}, fmt.Errorf("files are unavailable")
	}
	if h.deps.Accounts == nil {
		return mediaRequest{}, fmt.Errorf("accounts are unavailable")
	}
	if h.deps.Channels == nil {
		return mediaRequest{}, fmt.Errorf("channels are unavailable")
	}
	file, err := h.deps.Files.FindMediaByTelegramFileID(ctx, fileID)
	if err != nil {
		return mediaRequest{}, err
	}
	channel, err := h.deps.Channels.FindByID(ctx, file.ChannelID)
	if err != nil {
		return mediaRequest{}, err
	}
	account, err := h.deps.Accounts.FindByID(ctx, file.AccountID)
	if err != nil {
		return mediaRequest{}, err
	}
	if file.TelegramMessageID <= 0 {
		return mediaRequest{}, fmt.Errorf("message id is required")
	}
	return mediaRequest{
		session: h.accountSession(account),
		channel: telegram.MediaChannelRef{
			Username:          strings.TrimPrefix(channel.Username, "@"),
			TelegramChannelID: channel.TelegramChannelID,
			AccessHash:        channel.AccessHash,
			Type:              channel.Type,
		},
		messageID: int(file.TelegramMessageID),
		file:      file,
	}, nil
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

func (h handlers) markMediaAccountAuthFailure(ctx context.Context, session telegram.AccountSession, err error) {
	if h.deps.Accounts == nil || session.AccountID <= 0 || retry.Classify(err).Kind != retry.KindAuth {
		return
	}
	_ = h.deps.Accounts.UpdateStatus(ctx, session.AccountID, model.AccountStatusLoginRequired)
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
		end = start + maxOpenEndedVideoRangeLength - 1
		if end >= size {
			end = size - 1
		}
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
	case retry.Classify(err).Kind == retry.KindAuth:
		return http.StatusServiceUnavailable
	case strings.Contains(msg, "not found"), strings.Contains(msg, "no rows"), strings.Contains(msg, "has no"), strings.Contains(msg, "no usable photo size"):
		return http.StatusNotFound
	case strings.Contains(msg, "required"), strings.Contains(msg, "positive integer"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
