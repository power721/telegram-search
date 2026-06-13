package ai

import (
	"context"
	"errors"
	"testing"
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

type fakeProviderEnhancer struct {
	calls    int
	response EnhancementResponse
	err      error
}

func (f *fakeProviderEnhancer) Enhance(ctx context.Context, req EnhancementRequest) (EnhancementResponse, error) {
	f.calls++
	return f.response, f.err
}
