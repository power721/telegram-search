package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClientOptions struct {
	BaseURL            string
	APIKey             string
	Model              string
	AllowMissingAPIKey bool
	HTTPClient         *http.Client
}

type Client struct {
	baseURL            string
	apiKey             string
	model              string
	allowMissingAPIKey bool
	httpClient         *http.Client
}

type EnhancementRequest struct {
	Message EnhancementMessage `json:"message"`
	Links   []EnhancementLink  `json:"links"`
	Context map[string]any     `json:"context,omitempty"`
}

type EnhancementMessage struct {
	ID           int64  `json:"id"`
	Text         string `json:"text"`
	RawJSON      string `json:"raw_json,omitempty"`
	MessageType  string `json:"message_type,omitempty"`
	MediaSummary string `json:"media_summary,omitempty"`
}

type EnhancementLink struct {
	LinkID        int64         `json:"link_id"`
	Type          string        `json:"type"`
	URL           string        `json:"url"`
	Password      string        `json:"password,omitempty"`
	Note          string        `json:"note,omitempty"`
	SourceSnippet string        `json:"source_snippet,omitempty"`
	RawHint       string        `json:"raw_hint,omitempty"`
	Media         MediaMetadata `json:"media"`
}

type MediaMetadata struct {
	Title    string `json:"title,omitempty"`
	Year     string `json:"year,omitempty"`
	Season   string `json:"season,omitempty"`
	Episode  string `json:"episode,omitempty"`
	Quality  string `json:"quality,omitempty"`
	Size     string `json:"size,omitempty"`
	TMDBID   string `json:"tmdb_id,omitempty"`
	Category string `json:"category,omitempty"`
	Tags     string `json:"tags,omitempty"`
}

func (m *MediaMetadata) UnmarshalJSON(data []byte) error {
	var raw struct {
		Title    json.RawMessage `json:"title"`
		Year     json.RawMessage `json:"year"`
		Season   json.RawMessage `json:"season"`
		Episode  json.RawMessage `json:"episode"`
		Quality  json.RawMessage `json:"quality"`
		Size     json.RawMessage `json:"size"`
		TMDBID   json.RawMessage `json:"tmdb_id"`
		Category json.RawMessage `json:"category"`
		Tags     json.RawMessage `json:"tags"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var out MediaMetadata
	var err error
	if out.Title, err = decodeMetadataString(raw.Title); err != nil {
		return fmt.Errorf("title: %w", err)
	}
	if out.Year, err = decodeMetadataString(raw.Year); err != nil {
		return fmt.Errorf("year: %w", err)
	}
	if out.Season, err = decodeMetadataString(raw.Season); err != nil {
		return fmt.Errorf("season: %w", err)
	}
	if out.Episode, err = decodeMetadataString(raw.Episode); err != nil {
		return fmt.Errorf("episode: %w", err)
	}
	if out.Quality, err = decodeMetadataString(raw.Quality); err != nil {
		return fmt.Errorf("quality: %w", err)
	}
	if out.Size, err = decodeMetadataString(raw.Size); err != nil {
		return fmt.Errorf("size: %w", err)
	}
	if out.TMDBID, err = decodeMetadataString(raw.TMDBID); err != nil {
		return fmt.Errorf("tmdb_id: %w", err)
	}
	if out.Category, err = decodeMetadataString(raw.Category); err != nil {
		return fmt.Errorf("category: %w", err)
	}
	if out.Tags, err = decodeMetadataString(raw.Tags); err != nil {
		return fmt.Errorf("tags: %w", err)
	}
	*m = out
	return nil
}

func decodeMetadataString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var v any
	if err := decoder.Decode(&v); err != nil {
		return "", err
	}
	if num, ok := v.(json.Number); ok {
		return num.String(), nil
	}
	return "", fmt.Errorf("unsupported JSON value %s", string(raw))
}

type EnhancementResponse struct {
	Items []EnhancementItem `json:"items"`
}

type EnhancementItem struct {
	LinkID int64         `json:"link_id,omitempty"`
	URL    string        `json:"url,omitempty"`
	Media  MediaMetadata `json:"media"`
}

func NewClient(opts ClientOptions) *Client {
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:            strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
		apiKey:             strings.TrimSpace(opts.APIKey),
		model:              strings.TrimSpace(opts.Model),
		allowMissingAPIKey: opts.AllowMissingAPIKey,
		httpClient:         httpClient,
	}
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	if err := c.validateBase(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	models := make([]string, 0, len(body.Data))
	for _, item := range body.Data {
		if id := strings.TrimSpace(item.ID); id != "" {
			models = append(models, id)
		}
	}
	return models, nil
}

func (c *Client) Enhance(ctx context.Context, input EnhancementRequest) (EnhancementResponse, error) {
	if err := c.validateBase(); err != nil {
		return EnhancementResponse{}, err
	}
	if c.model == "" {
		return EnhancementResponse{}, errors.New("ai model is required")
	}
	payload, err := c.chatPayload(input)
	if err != nil {
		return EnhancementResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return EnhancementResponse{}, fmt.Errorf("create chat completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return EnhancementResponse{}, fmt.Errorf("chat completion request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return EnhancementResponse{}, err
	}
	var body struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return EnhancementResponse{}, fmt.Errorf("decode chat completion response: %w", err)
	}
	if len(body.Choices) == 0 {
		return EnhancementResponse{}, errors.New("chat completion response has no choices")
	}
	out, err := decodeEnhancementResponse(body.Choices[0].Message.Content)
	if err != nil {
		return EnhancementResponse{}, err
	}
	if err := ValidateEnhancementResponse(out, input); err != nil {
		return EnhancementResponse{}, err
	}
	return out, nil
}

func (c *Client) validateBase() error {
	if c.baseURL == "" {
		return errors.New("ai base_url is required")
	}
	if c.apiKey == "" && !c.allowMissingAPIKey {
		return errors.New("ai api_key is required")
	}
	if c.httpClient == nil {
		return errors.New("http client is required")
	}
	return nil
}

func (c *Client) authorize(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func (c *Client) chatPayload(input EnhancementRequest) ([]byte, error) {
	user, err := json.Marshal(chatInput{
		Task:  "enhance_media_metadata",
		Input: input,
	})
	if err != nil {
		return nil, fmt.Errorf("encode enhancement request: %w", err)
	}
	req := struct {
		Model          string         `json:"model"`
		Messages       []chatMessage  `json:"messages"`
		ResponseFormat map[string]any `json:"response_format,omitempty"`
	}{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: mediaMetadataSystemPrompt,
			},
			{Role: "user", Content: string(user)},
		},
		ResponseFormat: enhancementResponseFormat(),
	}
	return json.Marshal(req)
}

type chatInput struct {
	Task  string             `json:"task"`
	Input EnhancementRequest `json:"input"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const mediaMetadataSystemPrompt = `You are a strict data extraction engine.

You must output ONLY valid JSON.

The JSON MUST match this schema exactly:

{
  "items": [
    {
      "link_id": number,
      "url": string,
      "media": {
        "title": string|null,
        "year": string|null,
        "season": string|null,
        "episode": string|null,
        "quality": string|null,
        "size": string|null,
        "tmdb_id": string|null,
        "category": string|null,
        "tags": string|null
      }
    }
  ]
}

Rules:
- Do NOT add extra fields
- Do NOT omit "items"
- Do NOT invent metadata
- If unknown, use null
- Match items strictly by link_id or url
- Output JSON only, no markdown, no explanation`

func enhancementResponseFormat() map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "EnhancementResponse",
			"strict": true,
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"items"},
				"properties": map[string]any{
					"items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"link_id", "url", "media"},
							"properties": map[string]any{
								"link_id": map[string]any{"type": "number"},
								"url":     map[string]any{"type": "string"},
								"media":   mediaMetadataSchema(),
							},
						},
					},
				},
			},
		},
	}
}

func mediaMetadataSchema() map[string]any {
	fields := map[string]any{}
	for _, name := range []string{"title", "year", "season", "episode", "quality", "size", "tmdb_id", "category", "tags"} {
		fields[name] = map[string]any{"type": []string{"string", "null"}}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"title", "year", "season", "episode", "quality", "size", "tmdb_id", "category", "tags"},
		"properties":           fields,
	}
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(data))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("ai provider returned %d: %s", resp.StatusCode, msg)
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end >= start {
		return content[start : end+1]
	}
	return content
}
