package link

import (
	"regexp"
	"strings"

	"tg-provider/internal/model"
)

type Extractor struct {
	linkPattern     *regexp.Regexp
	passwordPattern *regexp.Regexp
}

func NewExtractor() *Extractor {
	return &Extractor{
		linkPattern:     regexp.MustCompile(`(?i)(https?://[^\s"'<>，。；；、]+|magnet:\?[^\s"'<>，。；；、]+|ed2k://[^\s"'<>，。；；、]+)`),
		passwordPattern: regexp.MustCompile(`(?i)(?:提取码|访问码|密码|code|pass|pwd)[：:\s]*([A-Za-z0-9]{2,8})`),
	}
}

func (e *Extractor) Extract(text string) []model.Link {
	if e == nil {
		e = NewExtractor()
	}
	matches := e.linkPattern.FindAllStringIndex(text, -1)
	seen := map[string]struct{}{}
	var out []model.Link
	for _, match := range matches {
		raw := strings.TrimSpace(text[match[0]:match[1]])
		url := trimTrailingPunctuation(raw)
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, model.Link{
			Type:     detectType(url),
			URL:      url,
			Password: e.nearbyPassword(text, match[1]),
		})
	}
	return out
}

func detectType(url string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.HasPrefix(lower, "magnet:?"):
		return "magnet"
	case strings.HasPrefix(lower, "ed2k://"):
		return "ed2k"
	default:
		return "url"
	}
}

func trimTrailingPunctuation(url string) string {
	return strings.TrimRight(url, ".,;:!?)]}）】》\"'")
}

func (e *Extractor) nearbyPassword(text string, after int) string {
	end := after + 80
	if end > len(text) {
		end = len(text)
	}
	segment := text[after:end]
	m := e.passwordPattern.FindStringSubmatch(segment)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}
