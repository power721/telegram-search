package ai

import "testing"

func TestProviderRegistryContainsPresetsAndFallbackOrder(t *testing.T) {
	providers := ProviderPresets()
	if len(providers) < 8 {
		t.Fatalf("providers len = %d, want at least 8", len(providers))
	}

	groq, ok := LookupProvider(ProviderGroq)
	if !ok {
		t.Fatal("Groq provider missing")
	}
	if groq.BaseURL != "https://api.groq.com/openai/v1" {
		t.Fatalf("groq base url = %q", groq.BaseURL)
	}
	if groq.ChatCompletionsURL() != "https://api.groq.com/openai/v1/chat/completions" {
		t.Fatalf("groq chat url = %q", groq.ChatCompletionsURL())
	}
	if groq.DefaultModel != "llama-3.3-70b-versatile" {
		t.Fatalf("groq default model = %q", groq.DefaultModel)
	}
	if groq.APIKeyEnv != "GROQ_API_KEY" {
		t.Fatalf("groq api key env = %q", groq.APIKeyEnv)
	}
	if groq.Website == "" {
		t.Fatal("groq website is empty")
	}

	ollama, ok := LookupProvider(ProviderOllama)
	if !ok {
		t.Fatal("Ollama provider missing")
	}
	if ollama.RequiresAPIKey {
		t.Fatal("ollama requires api key, want false")
	}

	order := DefaultFallbackProviders()
	want := []Provider{ProviderGroq, ProviderSiliconFlow, ProviderModelScope, ProviderOllama, ProviderOpenAI}
	if len(order) != len(want) {
		t.Fatalf("fallback len = %d, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("fallback[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}
