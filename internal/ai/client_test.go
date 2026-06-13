package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestClientListModelsUsesOpenAICompatibleEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1-mini"},{"id":"qwen-plus"}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientOptions{BaseURL: server.URL + "/v1", APIKey: "test-key", HTTPClient: server.Client()})
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if !reflect.DeepEqual(models, []string{"gpt-4.1-mini", "qwen-plus"}) {
		t.Fatalf("models = %#v", models)
	}
}

func TestClientEnhanceParsesChatCompletionJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "media-model" {
			t.Fatalf("model = %q, want media-model", req.Model)
		}
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "{\"items\":[{\"link_id\":12,\"url\":\"https://pan.quark.cn/s/a\",\"media\":{\"title\":\"迷墙\",\"year\":\"2026\"}}]}"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(ClientOptions{BaseURL: server.URL + "/v1", APIKey: "test-key", Model: "media-model", HTTPClient: server.Client()})
	resp, err := client.Enhance(context.Background(), EnhancementRequest{
		Message: EnhancementMessage{ID: 1, Text: "迷墙 https://pan.quark.cn/s/a"},
		Links: []EnhancementLink{
			{LinkID: 12, Type: "quark", URL: "https://pan.quark.cn/s/a"},
		},
	})
	if err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].LinkID != 12 || resp.Items[0].Media.Title != "迷墙" || resp.Items[0].Media.Year != "2026" {
		t.Fatalf("response = %+v", resp)
	}
}
