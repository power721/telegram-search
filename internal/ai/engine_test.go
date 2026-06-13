package ai

import (
	"context"
	"errors"
	"testing"

	"tg-search/internal/config"
)

func TestEngineFallsBackToFirstValidProvider(t *testing.T) {
	first := &fakeProviderEnhancer{err: errors.New("provider failed")}
	second := &fakeProviderEnhancer{response: EnhancementResponse{Items: []EnhancementItem{{
		LinkID: 12,
		Media:  MediaMetadata{Title: "迷墙"},
	}}}}
	engine := &Engine{providers: []providerEnhancer{
		{name: ProviderGroq, enhancer: first},
		{name: ProviderModelScope, enhancer: second},
	}}

	resp, err := engine.Enhance(context.Background(), EnhancementRequest{
		Links: []EnhancementLink{{LinkID: 12, URL: "https://pan.quark.cn/s/a"}},
	})
	if err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if resp.Items[0].Media.Title != "迷墙" {
		t.Fatalf("response = %+v", resp)
	}
	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("calls first=%d second=%d, want 1/1", first.calls, second.calls)
	}
}

func TestEngineSkipsInvalidProviderResponse(t *testing.T) {
	first := &fakeProviderEnhancer{response: EnhancementResponse{Items: []EnhancementItem{{Media: MediaMetadata{Title: "bad"}}}}}
	second := &fakeProviderEnhancer{response: EnhancementResponse{Items: []EnhancementItem{{LinkID: 12, Media: MediaMetadata{Title: "good"}}}}}
	engine := &Engine{providers: []providerEnhancer{
		{name: ProviderGroq, enhancer: first},
		{name: ProviderModelScope, enhancer: second},
	}}

	resp, err := engine.Enhance(context.Background(), EnhancementRequest{
		Links: []EnhancementLink{{LinkID: 12, URL: "https://pan.quark.cn/s/a"}},
	})
	if err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if resp.Items[0].Media.Title != "good" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestBuildProviderChainUsesConfiguredProviderListOrder(t *testing.T) {
	settings := config.AIMediaMetadataSettings{
		Enabled:         true,
		FallbackEnabled: true,
		Providers: []config.AIMediaMetadataProviderSettings{
			{ID: "ollama-local", Provider: "ollama", BaseURL: "http://localhost:11434/v1", Model: "qwen2.5:7b", Enabled: true},
			{ID: "groq-main", Provider: "groq", BaseURL: "https://api.groq.com/openai/v1", APIKey: "secret", Model: "llama-3.3-70b-versatile", Enabled: true},
			{ID: "disabled", Provider: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "secret", Model: "gpt-4o-mini", Enabled: false},
		},
	}

	chain := buildProviderChain(settings)

	if len(chain) != 2 {
		t.Fatalf("chain len = %d, want 2", len(chain))
	}
	if chain[0].name != ProviderOllama || chain[1].name != ProviderGroq {
		t.Fatalf("chain names = %q, %q; want ollama, groq", chain[0].name, chain[1].name)
	}
}

type fakeProviderEnhancer struct {
	calls    int
	response EnhancementResponse
	err      error
}

func (f *fakeProviderEnhancer) Enhance(ctx context.Context, req EnhancementRequest) (EnhancementResponse, error) {
	f.calls++
	return f.response, f.err
}
