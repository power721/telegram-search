package channel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"tg-search/internal/model"
	"tg-search/internal/repository"
	taskpkg "tg-search/internal/task"
)

const defaultWebAccessTimeout = 5 * time.Second
const maxWebAccessConcurrency = 5

type WebAccessChecker interface {
	Check(ctx context.Context, username string) (bool, error)
}

type WebAccessResult struct {
	ChannelID      int64     `json:"channel_id"`
	WebAccess      bool      `json:"web_access"`
	CheckedAt      time.Time `json:"checked_at"`
	WebAccessError string    `json:"web_access_error"`
}

type WebAccessService struct {
	channels *repository.ChannelRepository
	checker  WebAccessChecker
	now      func() time.Time
}

func NewWebAccessService(channels *repository.ChannelRepository, checker WebAccessChecker) *WebAccessService {
	if checker == nil {
		checker = NewHTTPWebAccessChecker(defaultWebAccessTimeout)
	}
	return &WebAccessService{
		channels: channels,
		checker:  checker,
		now:      time.Now,
	}
}

func (s *WebAccessService) CheckMany(ctx context.Context, channelIDs []int64) ([]WebAccessResult, error) {
	return s.CheckManyWithProgress(ctx, channelIDs, nil)
}

func (s *WebAccessService) CheckManyWithProgress(ctx context.Context, channelIDs []int64, progress taskpkg.ProgressSink) ([]WebAccessResult, error) {
	loaded, err := s.loadUniqueChannels(ctx, channelIDs)
	if err != nil {
		return nil, err
	}
	accesses := s.checkWebAccess(ctx, loaded)
	results := make([]WebAccessResult, 0, len(loaded))
	for i, item := range loaded {
		if err := checkProgressStatus(ctx, progress); err != nil {
			return results, err
		}
		result := accesses[i]
		checkedAt := s.now().UTC()
		if err := s.channels.UpdateWebAccessResult(ctx, item.ID, result.access, checkedAt, result.errorText); err != nil {
			return nil, err
		}
		results = append(results, WebAccessResult{
			ChannelID:      item.ID,
			WebAccess:      result.access,
			CheckedAt:      checkedAt,
			WebAccessError: result.errorText,
		})
		if progress != nil {
			if err := progress.Progress(ctx, int64(i+1), int64(len(loaded)), "web access result stored"); err != nil {
				return results, err
			}
		}
	}
	return results, nil
}

type webAccessCheck struct {
	access    bool
	errorText string
}

func (s *WebAccessService) checkWebAccess(ctx context.Context, channels []model.Channel) []webAccessCheck {
	accesses := make([]webAccessCheck, len(channels))
	sem := make(chan struct{}, maxWebAccessConcurrency)
	var wg sync.WaitGroup
	for i, item := range channels {
		wg.Add(1)
		go func(i int, item model.Channel) {
			defer wg.Done()
			username := strings.TrimPrefix(item.Username, "@")
			if item.Type == model.ChannelTypeSavedMessages || username == "" {
				return
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			checked, err := s.checker.Check(ctx, username)
			if err != nil {
				accesses[i] = webAccessCheck{access: false, errorText: err.Error()}
				return
			}
			accesses[i] = webAccessCheck{access: checked}
		}(i, item)
	}
	wg.Wait()
	return accesses
}

func (s *WebAccessService) loadUniqueChannels(ctx context.Context, channelIDs []int64) ([]model.Channel, error) {
	seen := map[int64]struct{}{}
	out := make([]model.Channel, 0, len(channelIDs))
	for _, id := range channelIDs {
		if id <= 0 {
			return nil, fmt.Errorf("channel id must be positive")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		channel, err := s.channels.FindByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("find channel %d: %w", id, err)
		}
		out = append(out, channel)
	}
	return out, nil
}

type HTTPWebAccessChecker struct {
	client  *http.Client
	baseURL string
}

func NewHTTPWebAccessChecker(timeout time.Duration) *HTTPWebAccessChecker {
	if timeout <= 0 {
		timeout = defaultWebAccessTimeout
	}
	return &HTTPWebAccessChecker{
		client:  &http.Client{Timeout: timeout},
		baseURL: "https://t.me",
	}
}

func (c *HTTPWebAccessChecker) Check(ctx context.Context, username string) (bool, error) {
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		return false, nil
	}
	baseURL := c.baseURL
	if baseURL == "" {
		baseURL = "https://t.me"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/s/"+url.PathEscape(username), nil)
	if err != nil {
		return false, err
	}
	q := req.URL.Query()
	q.Set("q", "")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("User-Agent", "tg-search/1.0")
	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		return false, fmt.Errorf("telegram web access returned HTTP %d", resp.StatusCode)
	}
	return hasTelegramMessageWrap(resp.Body), nil
}

func hasTelegramMessageWrap(r io.Reader) bool {
	doc, err := html.Parse(io.LimitReader(r, 4*1024*1024))
	if err != nil {
		return false
	}
	var walk func(*html.Node) bool
	walk = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "div" {
			if hasClass(n, "tgme_widget_message_wrap") {
				return true
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if walk(child) {
				return true
			}
		}
		return false
	}
	return walk(doc)
}

func hasClass(n *html.Node, className string) bool {
	for _, attr := range n.Attr {
		if attr.Key != "class" {
			continue
		}
		for _, item := range strings.Fields(attr.Val) {
			if item == className {
				return true
			}
		}
	}
	return false
}
