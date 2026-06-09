package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tg-search/internal/apikey"
	"tg-search/internal/model"
	"tg-search/internal/resource"
)

const searchMediaURLTTL = 24 * time.Hour

func (h handlers) shouldSignMediaURLs(c *gin.Context) bool {
	if h.hasAdminSession(c) {
		return false
	}
	return apiKeyFromRequest(c.Request) != ""
}

func (h handlers) attachMediaToSearchResults(ctx context.Context, items []model.SearchResult, signed bool) ([]model.SearchResult, error) {
	for i := range items {
		media, err := h.searchResultMedia(ctx, items[i].ChannelUsername, items[i].ChannelID, items[i].TelegramMessageID, items[i].MessageType, items[i].Files, signed)
		if err != nil {
			return nil, err
		}
		items[i].Media = media
	}
	return items, nil
}

func (h handlers) attachMediaToFileResults(ctx context.Context, items []model.FileResult, signed bool) ([]model.FileResult, error) {
	for i := range items {
		media, err := h.fileResultMedia(ctx, items[i].ChannelUsername, items[i].ChannelID, items[i].TelegramMessageID, items[i].File, signed)
		if err != nil {
			return nil, err
		}
		items[i].Media = media
	}
	return items, nil
}

func (h handlers) attachMediaToRemoteSearchResults(ctx context.Context, result model.RemoteSearchResults, signed bool) (model.RemoteSearchResults, error) {
	for i := range result.Items {
		media, err := h.searchResultMedia(ctx, result.Items[i].ChannelUsername, result.Items[i].ChannelID, result.Items[i].TelegramMessageID, result.Items[i].MessageType, result.Items[i].Files, signed)
		if err != nil {
			return model.RemoteSearchResults{}, err
		}
		result.Items[i].Media = media
	}
	return result, nil
}

func (h handlers) attachMediaToResourceItems(ctx context.Context, items []resource.Item, signed bool) ([]resource.Item, error) {
	for i := range items {
		media, err := h.resourceItemMedia(ctx, items[i], signed)
		if err != nil {
			return nil, err
		}
		items[i].Media = media
	}
	return items, nil
}

func (h handlers) searchResultMedia(ctx context.Context, username string, channelID int64, telegramMessageID int64, messageType string, files []model.File, signed bool) (*model.MediaURLs, error) {
	if telegramMessageID <= 0 {
		return nil, nil
	}
	hasVideo := messageType == "video"
	hasImage := messageType == "photo" || messageType == "video"
	for _, file := range files {
		if isVideoMedia(file.MimeType, file.Extension, file.FileName) {
			hasVideo = true
			hasImage = true
		}
		if isImageMedia(file.MimeType, file.Extension, file.FileName) {
			hasImage = true
		}
	}
	return h.mediaURLs(ctx, username, channelID, telegramMessageID, hasImage, hasVideo, signed)
}

func (h handlers) fileResultMedia(ctx context.Context, username string, channelID int64, telegramMessageID int64, file model.File, signed bool) (*model.MediaURLs, error) {
	if telegramMessageID <= 0 {
		return nil, nil
	}
	hasVideo := isVideoMedia(file.MimeType, file.Extension, file.FileName)
	hasImage := hasVideo || isImageMedia(file.MimeType, file.Extension, file.FileName)
	return h.mediaURLs(ctx, username, channelID, telegramMessageID, hasImage, hasVideo, signed)
}

func (h handlers) resourceItemMedia(ctx context.Context, item resource.Item, signed bool) (*model.MediaURLs, error) {
	if item.TelegramMessageID <= 0 {
		return nil, nil
	}
	hasVideo := false
	hasImage := item.MessageType == "photo" || item.MessageType == "video"
	if item.Kind == "file" {
		hasVideo = isVideoMedia(item.MimeType, item.Extension, item.FileName)
		hasImage = hasImage || hasVideo || isImageMedia(item.MimeType, item.Extension, item.FileName)
	}
	if item.MessageType == "video" {
		hasVideo = true
	}
	return h.mediaURLs(ctx, item.ChannelUsername, item.ChannelID, item.TelegramMessageID, hasImage, hasVideo, signed)
}

func (h handlers) mediaURLs(ctx context.Context, username string, channelID int64, telegramMessageID int64, hasImage bool, hasVideo bool, signed bool) (*model.MediaURLs, error) {
	if !hasImage && !hasVideo {
		return nil, nil
	}
	channel := mediaChannelParam(username, channelID)
	if channel == "" {
		return nil, nil
	}
	var media model.MediaURLs
	if hasImage {
		imageURL, err := h.mediaURL(ctx, "i", channel, telegramMessageID, signed)
		if err != nil {
			return nil, err
		}
		media.ImageURL = imageURL
	}
	if hasVideo {
		videoURL, err := h.mediaURL(ctx, "v", channel, telegramMessageID, signed)
		if err != nil {
			return nil, err
		}
		media.VideoURL = videoURL
	}
	if media.ImageURL == "" && media.VideoURL == "" {
		return nil, nil
	}
	return &media, nil
}

func mediaChannelParam(username string, channelID int64) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username != "" {
		return username
	}
	if channelID > 0 {
		return strconv.FormatInt(channelID, 10)
	}
	return ""
}

func (h handlers) mediaURL(ctx context.Context, kind string, channel string, telegramMessageID int64, signed bool) (string, error) {
	path := "/" + kind + "/" + url.PathEscape(channel) + "/" + strconv.FormatInt(telegramMessageID, 10)
	if !signed || h.deps.APIKeyService == nil {
		return path, nil
	}
	active, err := h.deps.APIKeyService.EnsureActive(ctx)
	if err != nil {
		return "", err
	}
	exp := strconv.FormatInt(time.Now().UTC().Add(searchMediaURLTTL).Unix(), 10)
	sig, err := apikey.MediaSignature(active.Key, http.MethodGet, path, exp)
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("exp", exp)
	values.Set("sig", sig)
	return path + "?" + values.Encode(), nil
}

func isVideoMedia(mimeType string, extension string, fileName string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mimeType, "video/") {
		return true
	}
	switch mediaExtension(extension, fileName) {
	case ".mp4", ".m4v", ".mkv", ".mov", ".avi", ".webm", ".flv", ".wmv", ".ts":
		return true
	default:
		return false
	}
}

func isImageMedia(mimeType string, extension string, fileName string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mimeType, "image/") {
		return true
	}
	switch mediaExtension(extension, fileName) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp":
		return true
	default:
		return false
	}
}

func mediaExtension(extension string, fileName string) string {
	extension = strings.ToLower(strings.TrimSpace(extension))
	if extension != "" {
		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		return extension
	}
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		return strings.ToLower(fileName[idx:])
	}
	return ""
}
