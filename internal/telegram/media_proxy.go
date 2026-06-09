package telegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

const (
	videoStreamPart = 512 * 1024
	imageStreamPart = 128 * 1024
)

func (g *GotdClient) VideoFile(ctx context.Context, session AccountSession, channel MediaChannelRef, messageID int) (VideoFile, error) {
	var out VideoFile
	err := g.withClient(ctx, session.SessionPath, func(ctx context.Context, client *gotdtelegram.Client) error {
		inputChannel, err := g.mediaInputChannel(ctx, client.API(), channel)
		if err != nil {
			return err
		}
		doc, err := getMessageDocument(ctx, client.API(), inputChannel, messageID)
		if err != nil {
			return err
		}
		out = videoFileFromDocument(doc)
		return nil
	})
	return out, err
}

func (g *GotdClient) StreamVideoRange(ctx context.Context, session AccountSession, channel MediaChannelRef, messageID int, file VideoFile, offset int64, length int64, w io.Writer) error {
	return g.withClient(ctx, session.SessionPath, func(ctx context.Context, client *gotdtelegram.Client) error {
		loc := documentFileLocation(file, "")
		written, err := streamFileRange(ctx, client.API(), w, loc, offset, length)
		if err == nil {
			return nil
		}
		if !isFileReferenceError(err) || written > 0 {
			return err
		}
		inputChannel, refreshErr := g.mediaInputChannel(ctx, client.API(), channel)
		if refreshErr != nil {
			return fmt.Errorf("%w; refresh file reference: %v", err, refreshErr)
		}
		doc, refreshErr := getMessageDocument(ctx, client.API(), inputChannel, messageID)
		if refreshErr != nil {
			return fmt.Errorf("%w; refresh file reference: %v", err, refreshErr)
		}
		_, err = streamFileRange(ctx, client.API(), w, documentFileLocation(videoFileFromDocument(doc), ""), offset, length)
		return err
	})
}

func (g *GotdClient) DownloadMessageImage(ctx context.Context, session AccountSession, channel MediaChannelRef, messageID int) (ImageFile, error) {
	var out ImageFile
	err := g.withClient(ctx, session.SessionPath, func(ctx context.Context, client *gotdtelegram.Client) error {
		inputChannel, err := g.mediaInputChannel(ctx, client.API(), channel)
		if err != nil {
			return err
		}
		msg, err := getMessage(ctx, client.API(), inputChannel, messageID)
		if err != nil {
			return err
		}
		out, err = downloadMessageImage(ctx, client.API(), msg)
		if err == nil || !isFileReferenceError(err) {
			return err
		}
		msg, refreshErr := getMessage(ctx, client.API(), inputChannel, messageID)
		if refreshErr != nil {
			return fmt.Errorf("%w; refresh file reference: %v", err, refreshErr)
		}
		out, err = downloadMessageImage(ctx, client.API(), msg)
		return err
	})
	return out, err
}

func (g *GotdClient) mediaInputChannel(ctx context.Context, api *tg.Client, channel MediaChannelRef) (*tg.InputChannel, error) {
	if channel.TelegramChannelID != 0 && channel.AccessHash != 0 {
		return &tg.InputChannel{
			ChannelID:  channel.TelegramChannelID,
			AccessHash: channel.AccessHash,
		}, nil
	}
	return resolveChannel(ctx, api, channel.Username)
}

func resolveChannel(ctx context.Context, api *tg.Client, username string) (*tg.InputChannel, error) {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return nil, fmt.Errorf("channel username is required")
	}
	resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, err
	}
	for _, c := range resolved.Chats {
		ch, ok := c.(*tg.Channel)
		if !ok {
			continue
		}
		accessHash, ok := ch.GetAccessHash()
		if !ok {
			continue
		}
		return &tg.InputChannel{
			ChannelID:  ch.ID,
			AccessHash: accessHash,
		}, nil
	}
	return nil, fmt.Errorf("channel not found: %s", username)
}

func getMessage(ctx context.Context, api *tg.Client, channel *tg.InputChannel, messageID int) (*tg.Message, error) {
	res, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: channel,
		ID: []tg.InputMessageClass{
			&tg.InputMessageID{ID: messageID},
		},
	})
	if err != nil {
		return nil, err
	}
	msgs, ok := res.(*tg.MessagesChannelMessages)
	if !ok || len(msgs.Messages) == 0 {
		return nil, fmt.Errorf("message not found")
	}
	msg, ok := msgs.Messages[0].(*tg.Message)
	if !ok {
		return nil, fmt.Errorf("unexpected message type %T", msgs.Messages[0])
	}
	return msg, nil
}

func getMessageDocument(ctx context.Context, api *tg.Client, channel *tg.InputChannel, messageID int) (*tg.Document, error) {
	msg, err := getMessage(ctx, api, channel, messageID)
	if err != nil {
		return nil, err
	}
	media, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil, fmt.Errorf("message has no document media")
	}
	doc, ok := media.Document.(*tg.Document)
	if !ok {
		return nil, fmt.Errorf("unexpected document type %T", media.Document)
	}
	return doc, nil
}

func videoFileFromDocument(doc *tg.Document) VideoFile {
	return VideoFile{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: append([]byte(nil), doc.FileReference...),
		Size:          doc.Size,
		MIMEType:      doc.MimeType,
	}
}

func documentFileLocation(file VideoFile, thumbSize string) *tg.InputDocumentFileLocation {
	return &tg.InputDocumentFileLocation{
		ID:            file.ID,
		AccessHash:    file.AccessHash,
		FileReference: file.FileReference,
		ThumbSize:     thumbSize,
	}
}

func streamFileRange(ctx context.Context, api *tg.Client, w io.Writer, loc tg.InputFileLocationClass, offset int64, remain int64) (int64, error) {
	var written int64
	for remain > 0 {
		limit := videoStreamPart
		if remain < int64(limit) {
			limit = int(remain)
		}
		res, err := api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: loc,
			Offset:   offset,
			Limit:    limit,
			Precise:  true,
		})
		if err != nil {
			return written, err
		}
		f, ok := res.(*tg.UploadFile)
		if !ok {
			return written, fmt.Errorf("unexpected upload.getFile result %T", res)
		}
		if len(f.Bytes) == 0 {
			return written, io.ErrUnexpectedEOF
		}
		if _, err := w.Write(f.Bytes); err != nil {
			return written, err
		}
		n := int64(len(f.Bytes))
		written += n
		offset += n
		remain -= n
		if n < int64(limit) && remain > 0 {
			return written, io.ErrUnexpectedEOF
		}
	}
	return written, nil
}

func downloadMessageImage(ctx context.Context, api *tg.Client, msg *tg.Message) (ImageFile, error) {
	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return ImageFile{}, fmt.Errorf("unexpected photo type %T", media.Photo)
		}
		return downloadPhotoImage(ctx, api, photo)
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return ImageFile{}, fmt.Errorf("unexpected document type %T", media.Document)
		}
		return downloadDocumentThumb(ctx, api, doc)
	case *tg.MessageMediaWebPage:
		webPage, ok := media.Webpage.(*tg.WebPage)
		if !ok {
			return ImageFile{}, fmt.Errorf("unexpected webpage type %T", media.Webpage)
		}
		if photo, ok := webPage.GetPhoto(); ok {
			if p, ok := photo.(*tg.Photo); ok {
				return downloadPhotoImage(ctx, api, p)
			}
			return ImageFile{}, fmt.Errorf("unexpected webpage photo type %T", photo)
		}
		if document, ok := webPage.GetDocument(); ok {
			if doc, ok := document.(*tg.Document); ok {
				return downloadDocumentThumb(ctx, api, doc)
			}
			return ImageFile{}, fmt.Errorf("unexpected webpage document type %T", document)
		}
		return ImageFile{}, fmt.Errorf("webpage has no image media")
	default:
		return ImageFile{}, fmt.Errorf("message has no image media")
	}
}

func downloadPhotoImage(ctx context.Context, api *tg.Client, photo *tg.Photo) (ImageFile, error) {
	thumbType, cached, err := choosePhotoSize(photo.Sizes)
	if err != nil {
		return ImageFile{}, err
	}
	if cached != nil {
		return ImageFile{Data: cached, MIMEType: http.DetectContentType(cached)}, nil
	}
	data, err := downloadSmallFile(ctx, api, &tg.InputPhotoFileLocation{
		ID:            photo.ID,
		AccessHash:    photo.AccessHash,
		FileReference: photo.FileReference,
		ThumbSize:     thumbType,
	})
	if err != nil {
		return ImageFile{}, err
	}
	return ImageFile{Data: data, MIMEType: imageMIME(data, "")}, nil
}

func downloadDocumentThumb(ctx context.Context, api *tg.Client, doc *tg.Document) (ImageFile, error) {
	thumbType, cached, err := choosePhotoSize(doc.Thumbs)
	if err != nil {
		return ImageFile{}, err
	}
	if cached != nil {
		return ImageFile{Data: cached, MIMEType: http.DetectContentType(cached)}, nil
	}
	data, err := downloadSmallFile(ctx, api, documentFileLocation(videoFileFromDocument(doc), thumbType))
	if err != nil {
		return ImageFile{}, err
	}
	return ImageFile{Data: data, MIMEType: imageMIME(data, doc.MimeType)}, nil
}

func choosePhotoSize(sizes []tg.PhotoSizeClass) (thumbType string, cached []byte, err error) {
	var bestArea int
	for _, s := range sizes {
		switch v := s.(type) {
		case *tg.PhotoCachedSize:
			area := v.W * v.H
			if area > bestArea && len(v.Bytes) > 0 {
				bestArea = area
				thumbType = v.Type
				cached = v.Bytes
			}
		case *tg.PhotoSize:
			area := v.W * v.H
			if area > bestArea {
				bestArea = area
				thumbType = v.Type
				cached = nil
			}
		case *tg.PhotoSizeProgressive:
			area := v.W * v.H
			if area > bestArea {
				bestArea = area
				thumbType = v.Type
				cached = nil
			}
		case *tg.PhotoStrippedSize:
			continue
		}
	}
	if thumbType == "" && cached == nil {
		return "", nil, fmt.Errorf("no usable photo size")
	}
	return thumbType, cached, nil
}

func downloadSmallFile(ctx context.Context, api *tg.Client, loc tg.InputFileLocationClass) ([]byte, error) {
	var out bytes.Buffer
	var offset int64
	for {
		res, err := api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: loc,
			Offset:   offset,
			Limit:    imageStreamPart,
			Precise:  true,
		})
		if err != nil {
			return nil, err
		}
		f, ok := res.(*tg.UploadFile)
		if !ok {
			return nil, fmt.Errorf("unexpected upload.getFile result %T", res)
		}
		if len(f.Bytes) == 0 {
			break
		}
		if _, err := out.Write(f.Bytes); err != nil {
			return nil, err
		}
		offset += int64(len(f.Bytes))
		if len(f.Bytes) < imageStreamPart {
			break
		}
	}
	if out.Len() == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return out.Bytes(), nil
}

func imageMIME(data []byte, fallback string) string {
	detected := http.DetectContentType(data)
	if detected != "application/octet-stream" {
		return detected
	}
	if fallback != "" {
		if ext, _, ok := strings.Cut(fallback, "/"); ok && ext == "image" {
			return fallback
		}
		if exts, _ := mime.ExtensionsByType(fallback); len(exts) > 0 && strings.HasPrefix(fallback, "image/") {
			return fallback
		}
		if byExt := mime.TypeByExtension(filepath.Ext(fallback)); byExt != "" && strings.HasPrefix(byExt, "image/") {
			return byExt
		}
	}
	return detected
}

func isFileReferenceError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "FILE_REFERENCE") || strings.Contains(msg, "FILEREF")
}
