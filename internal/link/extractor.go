package link

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"tg-search/internal/model"
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
			providerParser("115", `(?i)(https?://(?:115|115cdn|anxia)\.com/s/[\w-]+(?:\?password=([\w-]+))?)`, 1, 2),
			providerParser("xunlei", `(?i)(https?://pan\.xunlei\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("baidu", `(?i)(https?://pan\.baidu\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("baidu", `(?i)(https?://pan\.baidu\.com/(?:share|wap)/init\?surl=[\w-]+(?:&pwd=([\w-]+))?)`, 1, 2),
			providerParser("pikpak", `(?i)(https?://mypikpak\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("tianyi", `(?i)(https?://cloud\.189\.cn/web/share\?code=[\w-]+)`, 1, 0),
			providerParser("tianyi", `(?i)(https?://cloud\.189\.cn/t/[\w-]+)%EF%BC%88%E8%AE%BF%E9%97%AE%E7%A0%81%EF%BC%9A(\w+)%EF%BC%89`, 1, 2),
			providerParser("tianyi", `(?i)(https?://cloud\.189\.cn/t/[\w-]+(?:%[0-9A-Fa-f]{2})*)(?:（访问码：(\w+)）)?`, 1, 2),
			providerParser("tianyi", `(?i)(https?://h5\.cloud\.189\.cn/share\.html#/t/[\w-]+)`, 1, 0),
			providerParser("mobile", `(?i)(https?://(?:www\.)?caiyun\.139\.com/(?:m/i\?[\w-]+(?:&[\w%-]+=[\w%-]+)*|w/i/[\w-]+(?:\?[\w%-]+=[\w%-]+(?:&[\w%-]+=[\w%-]+)*)?))`, 1, 0),
			providerParser("mobile", `(?i)(https?://(?:www\.)?yun\.139\.com/shareweb/#/w/i/[\w-]+(?:\?[\w%-]+=[\w%-]+(?:&[\w%-]+=[\w%-]+)*)?)`, 1, 0),
			providerParser("mobile", `(?i)(https?://caiyun\.feixin\.10086\.cn/[\w-]+(?:\?[\w%-]+=[\w%-]+(?:&[\w%-]+=[\w%-]+)*)?)`, 1, 0),
			providerParser("quark", `(?i)(https?://pan\.quark\.cn/s/[\w-]+)`, 1, 0),
			providerParser("uc", `(?i)(https?://(?:drive|fast)\.uc\.cn/s/[\w-]+(?:\?[\w%-]+=[\w%-]+(?:&[\w%-]+=[\w%-]+)*)?)`, 1, 0),
			providerParser("aliyun", `(?i)(https?://(?:www\.)?(?:alipan|aliyundrive)\.com/s/[\w-]+(?:/folder/[\w-]+)?(?:\?password=([\w-]+))?)`, 1, 2),
			providerParser("123", `(?i)(https?://(?:www\.)?123(?:684|865|685|912|pan|592)\.(?:com|cn)/s/[\w-]+(?:\.html)?)(?:(?:\?(?:%E6%8F%90%E5%8F%96%E7%A0%81|提取码)|提取码)[:：](\w+))?`, 1, 2),
			providerParser("123", `(?i)(https?://[A-Za-z0-9-]+\.share\.123pan\.cn/123pan/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("guangya", `(?i)(https?://(?:www\.)?guangyapan\.com/s/[A-Za-z0-9_-]+)`, 1, 0),
			providerParser("magnet", `(?i)(magnet:\?[^\s"'<>，。；、]+)`, 1, 0),
			providerParser("ed2k", `(?i)(ed2k://\|file\|[^\r\n|]+\|\d+\|[A-Z0-9]+\|/)`, 1, 0),
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
	var providerSpans []matchSpan
	var out []model.Link
	for _, candidate := range candidates {
		url := trimTrailingPunctuation(strings.TrimSpace(candidate.URL))
		if url == "" {
			continue
		}
		if isIgnoredURL(url) {
			continue
		}
		if candidate.Type != "url" && overlapsSpan(candidate.MatchStart, candidate.MatchEnd, providerSpans) {
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
		note := inferNote(text, candidate.MatchStart)
		seen[url] = struct{}{}
		if candidate.Type != "url" {
			providerURLs = append(providerURLs, url)
			providerSpans = append(providerSpans, matchSpan{start: candidate.MatchStart, end: candidate.MatchEnd})
		}
		out = append(out, model.Link{
			Type:          candidate.Type,
			URL:           url,
			Password:      password,
			Note:          note,
			SourceSnippet: sourceSnippet(text, candidate.MatchStart, candidate.MatchEnd),
			Category:      resourceCategory(candidate.Type),
		})
	}
	return out
}

type matchSpan struct {
	start int
	end   int
}

func overlapsSpan(start int, end int, spans []matchSpan) bool {
	for _, item := range spans {
		if start < item.end && item.start < end {
			return true
		}
	}
	return false
}

func overlapsProvider(url string, providers []string) bool {
	for _, providerURL := range providers {
		if url == providerURL || strings.HasPrefix(url, providerURL) || strings.HasPrefix(providerURL, url) {
			return true
		}
	}
	return false
}

func resourceCategory(typ string) string {
	switch typ {
	case "magnet":
		return "magnet"
	case "ed2k":
		return "ed2k"
	case "url":
		return "http"
	default:
		return "cloud_drive"
	}
}

func sourceSnippet(text string, start int, end int) string {
	if start < 0 || start > len(text) {
		return ""
	}
	if end < start {
		end = start
	}
	lineStart := strings.LastIndex(text[:start], "\n") + 1
	lineEndRel := strings.Index(text[end:], "\n")
	lineEnd := len(text)
	if lineEndRel >= 0 {
		lineEnd = end + lineEndRel
	}
	snippet := strings.TrimSpace(text[lineStart:lineEnd])
	const maxSnippet = 240
	if utf8.RuneCountInString(snippet) <= maxSnippet {
		return snippet
	}
	runes := []rune(snippet)
	return string(runes[:maxSnippet])
}

func trimTrailingPunctuation(raw string) string {
	return strings.TrimRight(raw, ".,;:!?)]}）】》\"'，#")
}

func isIgnoredURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "t.me")
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

func inferNote(text string, linkStart int) string {
	if linkStart < 0 || linkStart > len(text) {
		return ""
	}
	lineStart := strings.LastIndex(text[:linkStart], "\n") + 1
	currentPrefix := text[lineStart:linkStart]
	if note := cleanNoteCandidate(currentPrefix); note != "" {
		return note
	}

	prevEnd := lineStart
	for prevEnd > 0 {
		prevStart := strings.LastIndex(text[:prevEnd-1], "\n") + 1
		line := text[prevStart : prevEnd-1]
		prevEnd = prevStart
		if strings.TrimSpace(line) == "" {
			continue
		}
		if note := cleanNoteCandidate(line); note != "" {
			return note
		}
	}
	return ""
}

func cleanNoteCandidate(raw string) string {
	candidate := strings.TrimSpace(raw)
	candidate = strings.TrimRight(candidate, ":：-—| \t")
	candidate = strings.TrimSpace(candidate)
	candidate = stripLeadingSymbols(candidate)
	for _, prefix := range []string{"名称", "标题", "片名", "电影", "电视剧", "剧集", "动漫", "动画", "综艺"} {
		switch {
		case strings.HasPrefix(candidate, prefix+"："):
			candidate = strings.TrimSpace(candidate[len(prefix)+len("："):])
		case strings.HasPrefix(candidate, prefix+":"):
			candidate = strings.TrimSpace(candidate[len(prefix)+len(":"):])
			break
		}
	}
	candidate = strings.TrimRight(candidate, ":：-—| \t")
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || isLinkLabel(candidate) || isMetadataLine(candidate) {
		return ""
	}
	return candidate
}

func stripLeadingSymbols(value string) string {
	for value != "" {
		r, size := utf8.DecodeRuneInString(value)
		if r == ' ' || r == '\t' || r == '-' || r == '*' || r == '>' || r == ':' || r == '：' || r == '|' {
			value = value[size:]
			continue
		}
		if isSymbolOrPunctuation(r) || unicode.IsMark(r) {
			value = value[size:]
			continue
		}
		break
	}
	return strings.TrimSpace(value)
}

func isSymbolOrPunctuation(r rune) bool {
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}

func isLinkLabel(value string) bool {
	normalized := strings.ToLower(strings.NewReplacer(" ", "", "\t", "", "：", "", ":", "", "-", "", "_", "", "网盘", "", "云盘", "").Replace(value))
	labels := map[string]struct{}{
		"":       {},
		"链接":     {},
		"地址":     {},
		"资源":     {},
		"资源地址":   {},
		"下载":     {},
		"网盘地址":   {},
		"夸克":     {},
		"quark":  {},
		"百度":     {},
		"baidu":  {},
		"阿里":     {},
		"aliyun": {},
		"alipan": {},
		"uc":     {},
		"迅雷":     {},
		"xunlei": {},
		"115":    {},
		"123":    {},
		"天翼":     {},
		"mobile": {},
		"pikpak": {},
		"磁力":     {},
		"ed2k":   {},
	}
	_, ok := labels[normalized]
	return ok
}

func isMetadataLine(value string) bool {
	head := value
	for _, separator := range []string{"：", ":"} {
		if idx := strings.Index(head, separator); idx >= 0 {
			head = head[:idx]
			break
		}
	}
	normalized := strings.ToLower(strings.NewReplacer(" ", "", "\t", "", "_", "", "-", "").Replace(strings.TrimSpace(head)))
	metadataLabels := map[string]struct{}{
		"tmdbid": {},
		"id":     {},
		"评分":     {},
		"类型":     {},
		"分类":     {},
		"质量":     {},
		"文件":     {},
		"大小":     {},
		"主演":     {},
		"简介":     {},
		"标签":     {},
	}
	_, ok := metadataLabels[normalized]
	return ok
}
