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
	documentMedia, ok := media.(*tg.MessageMediaDocument)
	if !ok {
		return nil
	}
	documentClass, ok := documentMedia.GetDocument()
	if !ok {
		return nil
	}
	document, ok := documentClass.(*tg.Document)
	if !ok {
		return nil
	}
	fileName := documentFileName(document)
	if fileName == "" {
		fileName = fmt.Sprintf("telegram-document-%d", document.ID)
	}
	return []model.File{{
		FileName:  fileName,
		Extension: strings.ToLower(filepath.Ext(fileName)),
		MimeType:  document.MimeType,
		SizeBytes: document.Size,
	}}
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
