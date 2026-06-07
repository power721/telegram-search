package link

import (
	"net/url"
	"regexp"
	"sort"
	"strings"

	"tg-provider/internal/model"
)

type Parser interface {
	Extract(text string) []Candidate
}

type Candidate struct {
	Type       string
	URL        string
	Password   string
	MatchStart int
	MatchEnd   int
}

type Extractor struct {
	parsers         []Parser
	passwordPattern *regexp.Regexp
}

type regexParser struct {
	typ       string
	pattern   *regexp.Regexp
	urlGroup  int
	passGroup int
}

func NewExtractor() *Extractor {
	return &Extractor{
		parsers: []Parser{
			providerParser("115", `(?i)(https://(?:115|115cdn|anxia)\.com/s/[\w-]+(?:\?password=([\w-]+))?)`, 1, 2),
			providerParser("xunlei", `(?i)(https://pan\.xunlei\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("baidu", `(?i)(https://pan\.baidu\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("baidu", `(?i)(https://pan\.baidu\.com/(?:share|wap)/init\?surl=[\w-]+(?:&pwd=([\w-]+))?)`, 1, 2),
			providerParser("pikpak", `(?i)(https://mypikpak\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("tianyi", `(?i)(https://cloud\.189\.cn/web/share\?code=[\w-]+)`, 1, 0),
			providerParser("tianyi", `(?i)(https://cloud\.189\.cn/t/[\w-]+)(?:（访问码：(\w+)）)?`, 1, 2),
			providerParser("tianyi", `(?i)(https://h5\.cloud\.189\.cn/share\.html#/t/[\w-]+)`, 1, 0),
			providerParser("mobile", `(?i)(https://caiyun\.139\.com/m/i\?[\w-]+)`, 1, 0),
			providerParser("mobile", `(?i)(https://yun\.139\.com/shareweb/#/w/i/[\w-]+)`, 1, 0),
			providerParser("mobile", `(?i)(https://caiyun\.139\.com/w/i/[\w-]+)`, 1, 0),
			providerParser("quark", `(?i)(https://pan\.quark\.cn/s/[\w-]+)`, 1, 0),
			providerParser("uc", `(?i)(https://(?:drive|fast)\.uc\.cn/s/[\w-]+(?:\?[^\s"'<>，。；、]*)?)`, 1, 0),
			providerParser("aliyun", `(?i)(https://www\.(?:alipan|aliyundrive)\.com/s/[\w-]+(?:/folder/[\w-]+)?(?:\?password=([\w-]+))?)`, 1, 2),
			providerParser("123", `(?i)(https://(?:www\.)?123[A-Za-z0-9]{3}\.com/s/[\w-]+(?:\.html)?)(?:\??提取码[:：](\w+))?`, 1, 2),
			providerParser("guangya", `(?i)(https://(?:www\.)?guangyapan\.com/s/[A-Za-z0-9_-]+)`, 1, 0),
			providerParser("magnet", `(?i)(magnet:\?[^\s"'<>，。；、]+)`, 1, 0),
			providerParser("ed2k", `(?i)(ed2k://[^\s"'<>，。；、]+)`, 1, 0),
			providerParser("url", `(?i)(https?://[^\s"'<>，。；、]+)`, 1, 0),
		},
		passwordPattern: regexp.MustCompile(`(?i)(?:密码|提取码|验证码|访问码|分享密码|密钥|pwd|password|code|share_pwd|pass_code|#)[=:：\s]*([A-Za-z0-9]{1,4})`),
	}
}

func providerParser(typ string, pattern string, urlGroup int, passGroup int) Parser {
	return regexParser{
		typ:       typ,
		pattern:   regexp.MustCompile(pattern),
		urlGroup:  urlGroup,
		passGroup: passGroup,
	}
}

func (p regexParser) Extract(text string) []Candidate {
	matches := p.pattern.FindAllStringSubmatchIndex(text, -1)
	out := make([]Candidate, 0, len(matches))
	for _, match := range matches {
		urlStart, urlEnd, ok := capture(match, p.urlGroup)
		if !ok {
			continue
		}
		candidate := Candidate{
			Type:       p.typ,
			URL:        text[urlStart:urlEnd],
			MatchStart: urlStart,
			MatchEnd:   urlEnd,
		}
		if passStart, passEnd, ok := capture(match, p.passGroup); ok {
			candidate.Password = text[passStart:passEnd]
		}
		out = append(out, candidate)
	}
	return out
}

func capture(match []int, group int) (int, int, bool) {
	if group <= 0 {
		return 0, 0, false
	}
	idx := group * 2
	if idx+1 >= len(match) || match[idx] < 0 || match[idx+1] < 0 {
		return 0, 0, false
	}
	return match[idx], match[idx+1], true
}

func (e *Extractor) Extract(text string) []model.Link {
	if e == nil {
		e = NewExtractor()
	}
	candidates := make([]Candidate, 0)
	for _, parser := range e.parsers {
		candidates = append(candidates, parser.Extract(text)...)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].MatchStart < candidates[j].MatchStart
	})

	seen := map[string]struct{}{}
	var providerURLs []string
	var out []model.Link
	for _, candidate := range candidates {
		url := trimTrailingPunctuation(strings.TrimSpace(candidate.URL))
		if url == "" {
			continue
		}
		if candidate.Type == "url" && overlapsProvider(url, providerURLs) {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		password := candidate.Password
		if password == "" {
			password = queryPassword(candidate.Type, url)
		}
		if password == "" {
			password = e.nearbyPassword(text, candidate.MatchEnd)
		}
		seen[url] = struct{}{}
		if candidate.Type != "url" {
			providerURLs = append(providerURLs, url)
		}
		out = append(out, model.Link{
			Type:     candidate.Type,
			URL:      url,
			Password: password,
		})
	}
	return out
}

func overlapsProvider(url string, providers []string) bool {
	for _, providerURL := range providers {
		if url == providerURL || strings.HasPrefix(url, providerURL) || strings.HasPrefix(providerURL, url) {
			return true
		}
	}
	return false
}

func trimTrailingPunctuation(raw string) string {
	return strings.TrimRight(raw, ".,;:!?)]}）】》\"'，#")
}

func queryPassword(typ string, raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	keys := []string{"pwd", "password", "share_pwd", "pass_code"}
	if typ != "tianyi" {
		keys = append(keys, "code")
	}
	values := parsed.Query()
	for _, key := range keys {
		if value := values.Get(key); value != "" {
			return value
		}
	}
	return ""
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
