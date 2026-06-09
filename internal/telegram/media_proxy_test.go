package telegram

import (
	"strings"
	"testing"

	"github.com/gotd/td/tg"
)

func TestDocumentImageSourceUsesOriginalImageDocument(t *testing.T) {
	doc := &tg.Document{
		ID:            42,
		AccessHash:    7,
		FileReference: []byte{1, 2, 3},
		MimeType:      "image/jpeg",
	}

	loc, fallbackMIME, cached, err := documentImageSource(doc)
	if err != nil {
		t.Fatalf("documentImageSource returned error: %v", err)
	}
	if cached != nil {
		t.Fatalf("cached = %v, want nil", cached)
	}
	if fallbackMIME != "image/jpeg" {
		t.Fatalf("fallback MIME = %q, want image/jpeg", fallbackMIME)
	}
	input, ok := loc.(*tg.InputDocumentFileLocation)
	if !ok {
		t.Fatalf("location = %T, want *tg.InputDocumentFileLocation", loc)
	}
	if input.ID != doc.ID || input.AccessHash != doc.AccessHash || input.ThumbSize != "" {
		t.Fatalf("location = %+v, want original document location without thumb size", input)
	}
}

func TestDocumentImageSourceUsesThumbnailForVideoDocument(t *testing.T) {
	doc := &tg.Document{
		ID:            42,
		AccessHash:    7,
		FileReference: []byte{1, 2, 3},
		MimeType:      "video/mp4",
		Thumbs: []tg.PhotoSizeClass{
			&tg.PhotoSize{Type: "m", W: 320, H: 180, Size: 4096},
		},
	}

	loc, fallbackMIME, cached, err := documentImageSource(doc)
	if err != nil {
		t.Fatalf("documentImageSource returned error: %v", err)
	}
	if cached != nil {
		t.Fatalf("cached = %v, want nil", cached)
	}
	if fallbackMIME != "video/mp4" {
		t.Fatalf("fallback MIME = %q, want video/mp4", fallbackMIME)
	}
	input, ok := loc.(*tg.InputDocumentFileLocation)
	if !ok {
		t.Fatalf("location = %T, want *tg.InputDocumentFileLocation", loc)
	}
	if input.ThumbSize != "m" {
		t.Fatalf("thumb size = %q, want m", input.ThumbSize)
	}
}

func TestDocumentImageSourceFallsBackToVideoThumbnail(t *testing.T) {
	doc := &tg.Document{
		ID:            42,
		AccessHash:    7,
		FileReference: []byte{1, 2, 3},
		MimeType:      "video/mp4",
		VideoThumbs: []tg.VideoSizeClass{
			&tg.VideoSize{Type: "v", W: 320, H: 180, Size: 8192},
		},
	}

	loc, fallbackMIME, cached, err := documentImageSource(doc)
	if err != nil {
		t.Fatalf("documentImageSource returned error: %v", err)
	}
	if cached != nil {
		t.Fatalf("cached = %v, want nil", cached)
	}
	if fallbackMIME != "video/mp4" {
		t.Fatalf("fallback MIME = %q, want video/mp4", fallbackMIME)
	}
	input, ok := loc.(*tg.InputDocumentFileLocation)
	if !ok {
		t.Fatalf("location = %T, want *tg.InputDocumentFileLocation", loc)
	}
	if input.ThumbSize != "v" {
		t.Fatalf("thumb size = %q, want v", input.ThumbSize)
	}
}

func TestDocumentImageSourceReportsMissingThumbnailForNonImageDocument(t *testing.T) {
	_, _, _, err := documentImageSource(&tg.Document{MimeType: "application/pdf"})
	if err == nil {
		t.Fatalf("documentImageSource returned nil error")
	}
	if !strings.Contains(err.Error(), "no usable photo size") {
		t.Fatalf("error = %q, want no usable photo size", err.Error())
	}
}
