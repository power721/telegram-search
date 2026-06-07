package link

import "testing"

func TestExtractFindsURLsAndNearbyPassword(t *testing.T) {
	extractor := NewExtractor()

	links := extractor.Extract("资源 https://www.aliyundrive.com/s/abc 提取码: 9x8y")

	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if links[0].Type != "url" {
		t.Fatalf("type = %q, want url", links[0].Type)
	}
	if links[0].URL != "https://www.aliyundrive.com/s/abc" {
		t.Fatalf("url = %q", links[0].URL)
	}
	if links[0].Password != "9x8y" {
		t.Fatalf("password = %q, want 9x8y", links[0].Password)
	}
}

func TestExtractFindsMagnetAndED2K(t *testing.T) {
	extractor := NewExtractor()

	links := extractor.Extract("magnet:?xt=urn:btih:abcdef ed2k://|file|movie.mkv|123|HASH|/")

	if len(links) != 2 {
		t.Fatalf("len = %d, want 2", len(links))
	}
	if links[0].Type != "magnet" {
		t.Fatalf("first type = %q, want magnet", links[0].Type)
	}
	if links[1].Type != "ed2k" {
		t.Fatalf("second type = %q, want ed2k", links[1].Type)
	}
}

func TestExtractDeduplicatesLinks(t *testing.T) {
	extractor := NewExtractor()

	links := extractor.Extract("https://example.com/a https://example.com/a")

	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
}
