package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"tg-search/internal/config"
)

type providerEnhancer struct {
	name     Provider
	enhancer Enhancer
}

type Engine struct {
	providers []providerEnhancer
}

func NewEngine(settings config.AIMediaMetadataSettings) *Engine {
	return &Engine{providers: buildProviderChain(settings)}
}

func (e *Engine) Enhance(ctx context.Context, req EnhancementRequest) (EnhancementResponse, error) {
	if e == nil || len(e.providers) == 0 {
		return EnhancementResponse{}, errors.New("no ai media metadata providers configured")
	}
	var failures []string
	for _, provider := range e.providers {
		resp, err := provider.enhancer.Enhance(ctx, req)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", provider.name, err))
			continue
		}
		if err := ValidateEnhancementResponse(resp, req); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", provider.name, err))
			continue
		}
		return resp, nil
	}
	return EnhancementResponse{}, fmt.Errorf("all ai media metadata providers failed: %s", strings.Join(failures, "; "))
}

func buildProviderChain(settings config.AIMediaMetadataSettings) []providerEnhancer {
	primaryID := NormalizeProvider(settings.Provider)
	ids := []Provider{primaryID}
	if settings.FallbackEnabled {
		for _, id := range DefaultFallbackProviders() {
			if id != primaryID {
				ids = append(ids, id)
			}
		}
	}

	chain := make([]providerEnhancer, 0, len(ids))
	for _, id := range ids {
		preset, ok := LookupProvider(id)
		if !ok {
			continue
		}
		baseURL := preset.BaseURL
		model := preset.DefaultModel
		apiKey := ""
		if id == primaryID {
			if strings.TrimSpace(settings.BaseURL) != "" {
				baseURL = settings.BaseURL
			}
			if strings.TrimSpace(settings.Model) != "" {
				model = settings.Model
			}
			apiKey = strings.TrimSpace(settings.APIKey)
		}
		if apiKey == "" && preset.APIKeyEnv != "" {
			apiKey = strings.TrimSpace(os.Getenv(preset.APIKeyEnv))
		}
		if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(model) == "" {
			continue
		}
		if preset.RequiresAPIKey && apiKey == "" {
			continue
		}
		chain = append(chain, providerEnhancer{
			name: id,
			enhancer: NewClient(ClientOptions{
				BaseURL:            baseURL,
				APIKey:             apiKey,
				Model:              model,
				AllowMissingAPIKey: !preset.RequiresAPIKey,
			}),
		})
	}
	return chain
}
