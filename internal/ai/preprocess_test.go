package ai

import (
	"testing"

	"tg-search/internal/model"
)

func TestPreProcessorExtractsMediaHints(t *testing.T) {
	hints := NewPreProcessor().Extract("阿凡达2 Avatar.The.Way.of.Water.2022 1080p WEB-DL S01E02 3.2GB")

	if hints["year"] != "2022" {
		t.Fatalf("year = %#v, want 2022", hints["year"])
	}
	if hints["quality"] != "1080p" {
		t.Fatalf("quality = %#v, want 1080p", hints["quality"])
	}
	if hints["season"] != "S01" {
		t.Fatalf("season = %#v, want S01", hints["season"])
	}
	if hints["episode"] != "E02" {
		t.Fatalf("episode = %#v, want E02", hints["episode"])
	}
	if hints["size"] != "3.2GB" {
		t.Fatalf("size = %#v, want 3.2GB", hints["size"])
	}
}

func TestBuildEnhancementRequestIncludesPreprocessedContextAndRawHints(t *testing.T) {
	req := buildEnhancementRequest(model.Message{ID: 1, Text: "沙丘2 2024 4K S01E03"}, []model.Link{
		{ID: 10, Type: "quark", URL: "https://pan.quark.cn/s/a", SourceSnippet: "沙丘2 2024 4K S01E03 12GB"},
	})

	if req.Context["year"] != "2024" {
		t.Fatalf("context year = %#v, want 2024", req.Context["year"])
	}
	if req.Context["quality"] != "4K" {
		t.Fatalf("context quality = %#v, want 4K", req.Context["quality"])
	}
	if len(req.Links) != 1 || req.Links[0].RawHint == "" {
		t.Fatalf("links = %+v, want raw_hint", req.Links)
	}
}
