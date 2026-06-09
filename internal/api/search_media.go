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
		media, err := h.searchResultMedia(ctx, items[i].ID, items[i].MessageType, items[i].Files, signed)
		if err != nil {
			return nil, err
		}
		items[i].Media = media
	}
	return items, nil
}

func (h handlers) attachMediaToFileResults(ctx context.Context, items []model.FileResult, signed bool) ([]model.FileResult, error) {
	for i := range items {
		media, err := h.fileResultMedia(ctx, items[i].File, signed)
		if err != nil {
			return nil, err
		}
		items[i].Media = media
	}
	return items, nil
}

func (h handlers) attachMediaToLinkResults(ctx context.Context, items []model.LinkResult, signed bool) ([]model.LinkResult, error) {
	for i := range items {
		media, err := h.searchResultMedia(ctx, items[i].MessageID, items[i].MessageType, nil, signed)
		if err != nil {
			return nil, err
		}
		items[i].Media = media
	}
	return items, nil
}

func (h handlers) attachMediaToRemoteSearchResults(ctx context.Context, result model.RemoteSearchResults, signed bool) (model.RemoteSearchResults, error) {
	for i := range result.Items {
		media, err := h.searchResultMedia(ctx, 0, result.Items[i].MessageType, result.Items[i].Files, signed)
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
		if media != nil {
			items[i].SetMediaURLs(media.ImageURL, media.VideoURL)
		}
	}
	return items, nil
}

func (h handlers) searchResultMedia(ctx context.Context, messageID int64, messageType string, files []model.File, signed bool) (*model.MediaURLs, error) {
	if len(files) == 0 && messageID > 0 && h.deps.Files != nil {
		found, err := h.deps.Files.FindByMessageID(ctx, messageID)
		if err != nil {
			return nil, err
		}
		files = found
	}
	var imageFileID int64
	var videoFileID int64
	for _, file := range files {
		if file.TelegramFileID <= 0 {
			continue
		}
		if videoFileID == 0 && isVideoMedia(file.MimeType, file.Extension, file.FileName) {
			videoFileID = file.TelegramFileID
		}
		if imageFileID == 0 && isImageMedia(file.MimeType, file.Extension, file.FileName) {
			imageFileID = file.TelegramFileID
		}
	}
	if videoFileID > 0 && imageFileID == 0 {
		imageFileID = videoFileID
	}
	if messageType == "photo" && imageFileID == 0 {
		imageFileID = firstTelegramFileID(files)
	}
	return h.mediaURLs(ctx, imageFileID, videoFileID, signed)
}

func (h handlers) fileResultMedia(ctx context.Context, file model.File, signed bool) (*model.MediaURLs, error) {
	if file.TelegramFileID <= 0 {
		return nil, nil
	}
	hasVideo := isVideoMedia(file.MimeType, file.Extension, file.FileName)
	hasImage := hasVideo || isImageMedia(file.MimeType, file.Extension, file.FileName)
	var imageFileID int64
	var videoFileID int64
	if hasImage {
		imageFileID = file.TelegramFileID
	}
	if hasVideo {
		videoFileID = file.TelegramFileID
	}
	return h.mediaURLs(ctx, imageFileID, videoFileID, signed)
}

func (h handlers) resourceItemMedia(ctx context.Context, item resource.Item, signed bool) (*model.MediaURLs, error) {
	files := []model.File{}
	if item.Kind == "file" {
		files = append(files, model.File{
			TelegramFileID: item.TelegramFileID,
			FileName:       item.FileName,
			Extension:      item.Extension,
			MimeType:       item.MimeType,
			SizeBytes:      item.SizeBytes,
			Category:       item.Category,
		})
	}
	if len(files) == 0 && item.ChannelID > 0 && item.TelegramMessageID > 0 && h.deps.Files != nil {
		found, err := h.deps.Files.FindByMessageRef(ctx, item.ChannelID, item.TelegramMessageID)
		if err != nil {
			return nil, err
		}
		files = found
	}
	return h.searchResultMedia(ctx, 0, item.MessageType, files, signed)
}

func (h handlers) mediaURLs(ctx context.Context, imageFileID int64, videoFileID int64, signed bool) (*model.MediaURLs, error) {
	if imageFileID <= 0 && videoFileID <= 0 {
		return nil, nil
	}
	var media model.MediaURLs
	if imageFileID > 0 {
		imageURL, err := h.mediaURL(ctx, "i", imageFileID, signed)
		if err != nil {
			return nil, err
		}
		media.ImageURL = imageURL
	}
	if videoFileID > 0 {
		videoURL, err := h.mediaURL(ctx, "v", videoFileID, signed)
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

func (h handlers) mediaURL(ctx context.Context, kind string, telegramFileID int64, signed bool) (string, error) {
	path := "/" + kind + "/" + url.PathEscape(strconv.FormatInt(telegramFileID, 10))
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

func firstTelegramFileID(files []model.File) int64 {
	for _, file := range files {
		if file.TelegramFileID > 0 {
			return file.TelegramFileID
		}
	}
	return 0
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
