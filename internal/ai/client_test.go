package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

func TestClientPingAcceptsAnyNonEmptyChatCompletionReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		var req struct {
			Model    string        `json:"model"`
			Messages []chatMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "media-model" {
			t.Fatalf("model = %q, want media-model", req.Model)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientOptions{BaseURL: server.URL + "/v1", APIKey: "test-key", Model: "media-model", HTTPClient: server.Client()})
	result, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if strings.TrimSpace(result.Content) != "pong" {
		t.Fatalf("content = %q, want pong", result.Content)
	}
}

func TestClientChatPayloadUsesStrictSchemaAndInstructionEnvelope(t *testing.T) {
	client := NewClient(ClientOptions{BaseURL: "https://api.example.com/v1", APIKey: "test-key", Model: "media-model"})
	payload, err := client.chatPayload(EnhancementRequest{
		Message: EnhancementMessage{ID: 1, Text: "迷墙 https://pan.quark.cn/s/a"},
		Links: []EnhancementLink{
			{LinkID: 12, Type: "quark", URL: "https://pan.quark.cn/s/a", Media: MediaMetadata{Title: "迷墙"}},
		},
	})
	if err != nil {
		t.Fatalf("chatPayload: %v", err)
	}

	var req struct {
		Model          string          `json:"model"`
		Messages       []chatMessage   `json:"messages"`
		ResponseFormat json.RawMessage `json:"response_format"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if req.Model != "media-model" {
		t.Fatalf("model = %q, want media-model", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "The JSON MUST match this schema exactly") {
		t.Fatalf("system prompt = %q, want strict schema contract", req.Messages[0].Content)
	}
	if !strings.Contains(req.Messages[0].Content, `- Do NOT add extra fields`) {
		t.Fatalf("system prompt = %q, want no-extra-fields rule", req.Messages[0].Content)
	}

	var user struct {
		Task  string             `json:"task"`
		Input EnhancementRequest `json:"input"`
	}
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &user); err != nil {
		t.Fatalf("decode user content: %v", err)
	}
	if user.Task != "enhance_media_metadata" {
		t.Fatalf("task = %q, want enhance_media_metadata", user.Task)
	}
	if len(user.Input.Links) != 1 || user.Input.Links[0].LinkID != 12 {
		t.Fatalf("input links = %+v, want original enhancement request", user.Input.Links)
	}

	var format struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name   string         `json:"name"`
			Strict bool           `json:"strict"`
			Schema map[string]any `json:"schema"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(req.ResponseFormat, &format); err != nil {
		t.Fatalf("decode response_format: %v", err)
	}
	if format.Type != "json_schema" || format.JSONSchema.Name != "EnhancementResponse" || !format.JSONSchema.Strict {
		t.Fatalf("response_format = %+v, want strict EnhancementResponse json_schema", format)
	}
	properties, ok := format.JSONSchema.Schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %#v, want object", format.JSONSchema.Schema["properties"])
	}
	if _, ok := properties["items"]; !ok {
		t.Fatalf("schema properties = %#v, want items", properties)
	}
}

func TestEnhancementResponseCoercesNullableAndNumericMediaFields(t *testing.T) {
	var resp EnhancementResponse
	err := json.Unmarshal([]byte(`{"items":[{"link_id":12,"url":"https://pan.quark.cn/s/a","media":{"title":"迷墙","season":null,"episode":1,"tmdb_id":123,"tags":null}}]}`), &resp)
	if err != nil {
		t.Fatalf("Unmarshal EnhancementResponse: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(resp.Items))
	}
	media := resp.Items[0].Media
	if media.Season != "" || media.Episode != "1" || media.TMDBID != "123" || media.Tags != "" {
		t.Fatalf("media = %+v, want nulls as empty strings and numbers as strings", media)
	}
}
