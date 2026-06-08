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

func documentFileName(document *tg.Document) string {
	for _, attr := range document.Attributes {
		if filename, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return strings.TrimSpace(filename.FileName)
		}
	}
	return ""
}
