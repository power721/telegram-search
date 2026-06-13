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
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type EnhancementRequest struct {
	Message EnhancementMessage `json:"message"`
	Links   []EnhancementLink  `json:"links"`
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
		baseURL:    strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
		apiKey:     strings.TrimSpace(opts.APIKey),
		model:      strings.TrimSpace(opts.Model),
		httpClient: httpClient,
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
	content := extractJSONObject(body.Choices[0].Message.Content)
	var out EnhancementResponse
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return EnhancementResponse{}, fmt.Errorf("decode media metadata json: %w", err)
	}
	if out.Items == nil {
		out.Items = []EnhancementItem{}
	}
	return out, nil
}

func (c *Client) validateBase() error {
	if c.baseURL == "" {
		return errors.New("ai base_url is required")
	}
	if c.apiKey == "" {
		return errors.New("ai api_key is required")
	}
	if c.httpClient == nil {
		return errors.New("http client is required")
	}
	return nil
}

func (c *Client) authorize(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}

func (c *Client) chatPayload(input EnhancementRequest) ([]byte, error) {
	user, err := json.Marshal(input)
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
				Content: "Fix media metadata for cloud-drive links. Return JSON only with an items array. Match items by link_id or url. Do not invent unrelated links.",
			},
			{Role: "user", Content: string(user)},
		},
		ResponseFormat: map[string]any{"type": "json_object"},
	}
	return json.Marshal(req)
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
