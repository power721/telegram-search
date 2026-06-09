package telegram

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gotd/td/tg"

	"tg-search/internal/model"
)

func FilesFromMessage(message *tg.Message) []model.File {
	if message == nil {
		return nil
	}
	media, ok := message.GetMedia()
	if !ok {
		return nil
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := m.Photo.(*tg.Photo)
		if !ok {
			return nil
		}
		return []model.File{{
			TelegramFileID: photo.ID,
			FileName:       fmt.Sprintf("telegram-photo-%d.jpg", photo.ID),
			Extension:      ".jpg",
			MimeType:       "image/jpeg",
			SizeBytes:      photoSizeBytes(photo),
			Category:       "image",
		}}
	case *tg.MessageMediaDocument:
		documentClass, ok := m.GetDocument()
		if !ok {
			return nil
		}
		document, ok := documentClass.(*tg.Document)
		if !ok {
			return nil
		}
		fileName := documentFileName(document)
		if fileName == "" {
			fileName = defaultDocumentFileName(document)
		}
		return []model.File{{
			TelegramFileID: document.ID,
			FileName:       fileName,
			Extension:      strings.ToLower(filepath.Ext(fileName)),
			MimeType:       document.MimeType,
			SizeBytes:      document.Size,
			Category:       mediaFileCategory(document),
		}}
	default:
		return nil
	}
}

func MessageMediaMetadata(message *tg.Message) (string, string) {
	if message == nil {
		return "text", ""
	}
	media, ok := message.GetMedia()
	if !ok {
		return "text", ""
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo", "photo"
	case *tg.MessageMediaDocument:
		doc, ok := m.GetDocument()
		if !ok {
			return "file", "document"
		}
		document, ok := doc.(*tg.Document)
		if !ok {
			return "file", "document"
		}
		if isVideoDocument(document) {
			return "video", document.MimeType
		}
		if isAudioDocument(document) {
			return "audio", document.MimeType
		}
		if strings.HasPrefix(strings.ToLower(document.MimeType), "image/") {
			return "photo", document.MimeType
		}
		return "file", document.MimeType
	case *tg.MessageMediaWebPage:
		webPage, ok := m.Webpage.(*tg.WebPage)
		if !ok {
			return "webpage", "webpage"
		}
		if photo, ok := webPage.GetPhoto(); ok && photo != nil {
			return "photo", "webpage_photo"
		}
		if doc, ok := webPage.GetDocument(); ok {
			if document, ok := doc.(*tg.Document); ok {
				if isVideoDocument(document) {
					return "video", "webpage_" + document.MimeType
				}
				if isAudioDocument(document) {
					return "audio", "webpage_" + document.MimeType
				}
				if strings.HasPrefix(strings.ToLower(document.MimeType), "image/") || len(document.Thumbs) > 0 {
					return "photo", "webpage_" + document.MimeType
				}
			}
		}
		return "webpage", "webpage"
	default:
		return "media", fmt.Sprintf("%T", media)
	}
}

func documentFileName(document *tg.Document) string {
	for _, attr := range document.Attributes {
		if filename, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return strings.TrimSpace(filename.FileName)
		}
	}
	return ""
}

func defaultDocumentFileName(document *tg.Document) string {
	switch {
	case isVideoDocument(document):
		return fmt.Sprintf("telegram-video-%d%s", document.ID, mimeExtension(document.MimeType, ".mp4"))
	case isAudioDocument(document):
		return fmt.Sprintf("telegram-audio-%d%s", document.ID, mimeExtension(document.MimeType, ".mp3"))
	case strings.HasPrefix(strings.ToLower(document.MimeType), "image/"):
		return fmt.Sprintf("telegram-image-%d%s", document.ID, mimeExtension(document.MimeType, ".jpg"))
	default:
		return fmt.Sprintf("telegram-document-%d", document.ID)
	}
}

func mediaFileCategory(document *tg.Document) string {
	switch {
	case isVideoDocument(document):
		return "video"
	case isAudioDocument(document):
		return "audio"
	case strings.HasPrefix(strings.ToLower(document.MimeType), "image/"):
		return "image"
	default:
		return ""
	}
}

func mimeExtension(mimeType string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4":
		return ".m4a"
	case "audio/ogg":
		return ".ogg"
	case "audio/opus":
		return ".opus"
	case "audio/flac":
		return ".flac"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	default:
		return fallback
	}
}

func photoSizeBytes(photo *tg.Photo) int64 {
	var best int64
	for _, size := range photo.Sizes {
		switch s := size.(type) {
		case *tg.PhotoSize:
			if int64(s.Size) > best {
				best = int64(s.Size)
			}
		case *tg.PhotoSizeProgressive:
			for _, size := range s.Sizes {
				if int64(size) > best {
					best = int64(size)
				}
			}
		case *tg.PhotoCachedSize:
			if int64(len(s.Bytes)) > best {
				best = int64(len(s.Bytes))
			}
		}
	}
	return best
}

func isVideoDocument(document *tg.Document) bool {
	if document == nil {
		return false
	}
	if strings.HasPrefix(strings.ToLower(document.MimeType), "video/") {
		return true
	}
	for _, attr := range document.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeVideo); ok {
			return true
		}
	}
	return false
}

func isAudioDocument(document *tg.Document) bool {
	if document == nil {
		return false
	}
	if strings.HasPrefix(strings.ToLower(document.MimeType), "audio/") {
		return true
	}
	for _, attr := range document.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeAudio); ok {
			return true
		}
	}
	return false
}
