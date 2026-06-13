package ai

import (
	"strings"
	"testing"
)

func TestDecodeEnhancementResponseRepairsTrailingCommaJSON(t *testing.T) {
	resp, err := decodeEnhancementResponse("```json\n{\"items\":[{\"link_id\":12,\"url\":\"https://pan.quark.cn/s/a\",\"media\":{\"title\":\"迷墙\",},}],}\n```")
	if err != nil {
		t.Fatalf("decodeEnhancementResponse: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Media.Title != "迷墙" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestValidateEnhancementResponseRejectsMissingIdentifier(t *testing.T) {
	err := ValidateEnhancementResponse(EnhancementResponse{Items: []EnhancementItem{{Media: MediaMetadata{Title: "迷墙"}}}}, EnhancementRequest{
		Links: []EnhancementLink{{LinkID: 12, URL: "https://pan.quark.cn/s/a"}},
	})
	if err == nil || !strings.Contains(err.Error(), "missing identifier") {
		t.Fatalf("ValidateEnhancementResponse error = %v, want missing identifier", err)
	}
}

func TestValidateEnhancementResponseRejectsUnknownLink(t *testing.T) {
	err := ValidateEnhancementResponse(EnhancementResponse{Items: []EnhancementItem{{
		LinkID: 99,
		URL:    "https://pan.quark.cn/s/unknown",
		Media:  MediaMetadata{Title: "迷墙"},
	}}}, EnhancementRequest{
		Links: []EnhancementLink{{LinkID: 12, URL: "https://pan.quark.cn/s/a"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown link") {
		t.Fatalf("ValidateEnhancementResponse error = %v, want unknown link", err)
	}
}
