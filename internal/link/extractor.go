package link

import (
	"fmt"
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

type mediaMetadata struct {
	Title    string
	Year     string
	Season   string
	Episode  string
	Quality  string
	Size     string
	TMDBID   string
	Category string
	Tags     string
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
	messageMetadata := extractMediaMetadata(text)
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
		metadata := messageMetadata
		metadata.merge(mediaMetadataFromURL(candidate.Type, url))
		if metadata.Title == "" {
			metadata.Title = note
		}
		if note == "" || isLowConfidenceNote(note) {
			note = metadata.Title
		}
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
			MediaTitle:    metadata.Title,
			MediaYear:     metadata.Year,
			MediaSeason:   metadata.Season,
			MediaEpisode:  metadata.Episode,
			MediaQuality:  metadata.Quality,
			MediaSize:     metadata.Size,
			MediaTMDBID:   metadata.TMDBID,
			MediaCategory: metadata.Category,
			MediaTags:     metadata.Tags,
		})
	}
	return out
}

func (m *mediaMetadata) merge(other mediaMetadata) {
	if m.Title == "" {
		m.Title = other.Title
	}
	if m.Year == "" {
		m.Year = other.Year
	}
	if m.Season == "" {
		m.Season = other.Season
	}
	if m.Episode == "" {
		m.Episode = other.Episode
	}
	if m.Quality == "" {
		m.Quality = other.Quality
	}
	if m.Size == "" {
		m.Size = other.Size
	}
	if m.TMDBID == "" {
		m.TMDBID = other.TMDBID
	}
	if m.Category == "" {
		m.Category = other.Category
	}
	if m.Tags == "" {
		m.Tags = other.Tags
	}
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
	host := strings.ToLower(parsed.Hostname())
	return host == "t.me" || host == "toapp.mypikpak.com"
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
	for _, prefix := range []string{"资源名称", "名称", "标题", "片名", "电影", "电视剧", "剧集", "动漫", "动画", "综艺"} {
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
		"描述":     {},
		"主演":     {},
		"简介":     {},
		"分享":     {},
		"来自":     {},
		"频道":     {},
		"群组":     {},
		"投稿":     {},
		"提取码":    {},
		"访问码":    {},
		"标签":     {},
	}
	_, ok := metadataLabels[normalized]
	return ok
}

func isLowConfidenceNote(note string) bool {
	if note == "" {
		return true
	}
	if isMetadataLine(note) || isLinkLabel(note) {
		return true
	}
	normalized := strings.ToLower(strings.TrimSpace(note))
	if strings.Contains(normalized, "://") || strings.Contains(normalized, "magnet:?") {
		return true
	}
	if strings.HasPrefix(normalized, "链接") || strings.HasPrefix(normalized, "直达链接") {
		return true
	}
	return strings.HasSuffix(normalized, "(") || strings.HasSuffix(normalized, "（")
}

func extractMediaMetadata(text string) mediaMetadata {
	var metadata mediaMetadata
	lines := strings.Split(text, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		clean := cleanMediaLine(line)
		if clean == "" {
			continue
		}
		hasResourceURL := isResourceURLLine(clean)
		if metadata.TMDBID == "" {
			metadata.TMDBID = extractFirstMatch(clean, `(?i)(?:TMDB(?:\s*ID)?|tmdb)[：:\s-]*(\d+)`)
			if metadata.TMDBID == "" {
				metadata.TMDBID = extractFirstMatch(clean, `(?i)\{tmdb-(\d+)\}`)
			}
		}
		if metadata.Size == "" {
			metadata.Size = extractFirstMatch(clean, `(?i)(?:大小|文件大小|体积|总大小)[：:\s]*([0-9]+(?:\.[0-9]+)?\s*(?:KB|MB|GB|TB|T))`)
		}
		if metadata.Quality == "" {
			metadata.Quality = extractLabeledValue(clean, []string{"质量", "视频质量"})
		}
		if category := extractLabeledValue(clean, []string{"分类"}); category != "" {
			metadata.Category = category
		}
		if metadata.Tags == "" {
			metadata.Tags = extractTags(clean)
		}
		if hasResourceURL {
			continue
		}
		if metadata.Category == "" {
			metadata.Category = categoryFromLine(clean)
		}
		if metadata.Title == "" {
			title, category := titleFromExplicitLine(clean)
			if title == "" {
				title, category = titleFromPlainLine(clean)
			}
			if title != "" {
				metadata.Title = title
				if metadata.Category == "" {
					metadata.Category = category
				}
			}
		}
		metadata.merge(sequenceMetadata(clean))
		if metadata.Year == "" {
			metadata.Year = extractYear(clean)
		}
		if metadata.Quality == "" {
			metadata.Quality = qualityFromLine(clean)
		}
	}
	if metadata.Title != "" {
		if metadata.Year == "" {
			metadata.Year = extractYear(metadata.Title)
		}
		metadata.merge(sequenceMetadata(metadata.Title))
		metadata.Title = normalizeMediaTitle(metadata.Title)
	}
	return metadata
}

func cleanMediaLine(line string) string {
	line = strings.TrimSpace(line)
	for line != "" {
		r, size := utf8.DecodeRuneInString(line)
		if r == ' ' || r == '\t' || r == '-' || r == '*' || r == '>' || r == '|' || unicode.IsSymbol(r) || unicode.IsMark(r) {
			line = strings.TrimSpace(line[size:])
			continue
		}
		break
	}
	return strings.TrimSpace(line)
}

func isResourceURLLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "magnet:?") || strings.Contains(lower, "ed2k://")
}

func titleFromExplicitLine(line string) (string, string) {
	category := ""
	if match := regexp.MustCompile(`^《([^》]+)》\s*(.+)$`).FindStringSubmatch(line); len(match) == 3 {
		category = strings.TrimSpace(match[1])
		return normalizeMediaTitle(match[2]), category
	}
	if match := regexp.MustCompile(`^\[([^\]]+)\]\s*(.+)$`).FindStringSubmatch(line); len(match) == 3 {
		category = strings.TrimSpace(match[1])
		return normalizeMediaTitle(match[2]), category
	}
	if match := regexp.MustCompile(`^(资源名称|名称|标题|片名|电影|电视剧|剧集|动漫|动画|综艺|短剧|已更新)\s*[：:]\s*(.+)$`).FindStringSubmatch(line); len(match) == 3 {
		if match[1] != "资源名称" && match[1] != "名称" && match[1] != "标题" && match[1] != "片名" && match[1] != "已更新" {
			category = match[1]
		}
		return normalizeMediaTitle(match[2]), category
	}
	if match := regexp.MustCompile(`^(电影|电视剧|剧集|动漫|动画|综艺|短剧)\s+(.+)$`).FindStringSubmatch(line); len(match) == 3 {
		return normalizeMediaTitle(match[2]), match[1]
	}
	if match := regexp.MustCompile(`^(短剧)[-—]\s*(.+)$`).FindStringSubmatch(line); len(match) == 3 {
		return normalizeMediaTitle(match[2]), match[1]
	}
	return "", ""
}

func titleFromPlainLine(line string) (string, string) {
	lower := strings.ToLower(line)
	if strings.Contains(line, "://") || strings.Contains(lower, "magnet:?") || isLinkLabel(line) || isMetadataLine(line) {
		return "", ""
	}
	if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@") {
		return "", ""
	}
	if utf8.RuneCountInString(line) > 80 {
		return "", ""
	}
	normalized := normalizeMediaTitle(line)
	if normalized == "" || isLinkLabel(normalized) || isMetadataLine(normalized) {
		return "", ""
	}
	if !looksLikeMediaTitle(line) {
		return "", ""
	}
	return normalized, ""
}

func categoryFromLine(line string) string {
	for _, category := range []string{"短剧", "综艺", "电视剧", "剧集", "电影", "动漫", "动画"} {
		if strings.Contains(line, category) {
			return category
		}
	}
	return ""
}

func looksLikeMediaTitle(line string) bool {
	if regexp.MustCompile(`(?:19|20)\d{2}|S\d{1,2}E?\d*|第[一二三四五六七八九十\d]+季|第\s*\d+\s*集|更新\s*\d+|\d+\s*集`).MatchString(line) {
		return true
	}
	for _, r := range line {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func normalizeMediaTitle(raw string) string {
	title := strings.TrimSpace(raw)
	title = strings.ReplaceAll(title, "｜", "|")
	if idx := strings.Index(title, "|"); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}
	if idx := regexp.MustCompile(`（\s*\d+\s*集\s*）|\(\s*\d+\s*集\s*\)`).FindStringIndex(title); idx != nil {
		title = title[:idx[0]]
	}
	title = regexp.MustCompile(`(?i)\{tmdb-\d+\}`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`(?i)\bTMDB(?:\s*ID)?[：:\s-]*\d+`).ReplaceAllString(title, "")
	if idx := regexp.MustCompile(`\s+[-—]\s+S\d{1,2}E\d{1,4}\b`).FindStringIndex(title); idx != nil {
		title = title[:idx[0]]
	}
	if match := regexp.MustCompile(`^(.*?)(?:\s*[（(](?:19|20)\d{2}[）)]).*$`).FindStringSubmatch(title); len(match) == 2 {
		title = match[1]
	}
	if idx := regexp.MustCompile(`\s+(?:19|20)\d{2}\b`).FindStringIndex(title); idx != nil {
		title = title[:idx[0]]
	}
	if idx := regexp.MustCompile(`(?i)\s+(?:WEB[- ]?(?:DL|4K)?|4K|8K|2160p|1080p|720p|BDISO|BluRay|REMUX|UHD|HDR10?|DV|SDR|DDP|DTS|HEVC|H\.?26[45]|完结|更新\s*\d+|第\s*\d+\s*集|第\s*\d+\s*期)\b`).FindStringIndex(title); idx != nil {
		title = title[:idx[0]]
	}
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	return strings.Trim(title, " \t:：-—,，")
}

func sequenceMetadata(line string) mediaMetadata {
	var metadata mediaMetadata
	if match := regexp.MustCompile(`(?i)\bS(\d{1,2})(?:E(\d{1,4}))?\b`).FindStringSubmatch(line); len(match) >= 2 {
		metadata.Season = "S" + zeroPad(match[1], 2)
		if len(match) >= 3 && match[2] != "" {
			metadata.Episode = "E" + zeroPad(match[2], 2)
		}
	}
	if metadata.Season == "" {
		if season := extractFirstMatch(line, `第([一二三四五六七八九十\d]+)季`); season != "" {
			metadata.Season = "第" + season + "季"
		}
	}
	if metadata.Episode == "" {
		if episode := extractFirstMatch(line, `第\s*(\d+)\s*集`); episode != "" {
			metadata.Episode = "E" + zeroPad(episode, 2)
		} else if episode := extractFirstMatch(line, `更新\s*(\d+)`); episode != "" {
			metadata.Episode = "更新" + episode
		} else if episode := extractFirstMatch(line, `(\d+)\s*集`); episode != "" {
			metadata.Episode = episode + "集"
		} else if episode := extractFirstMatch(line, `(\d{4})\s*期`); episode != "" {
			metadata.Episode = episode + "期"
		}
	}
	return metadata
}

func zeroPad(value string, width int) string {
	for len(value) < width {
		value = "0" + value
	}
	return value
}

func extractLabeledValue(line string, labels []string) string {
	for _, label := range labels {
		pattern := `^` + regexp.QuoteMeta(label) + `[：:]\s*(.+)$`
		if match := regexp.MustCompile(pattern).FindStringSubmatch(line); len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func extractTags(line string) string {
	value := extractLabeledValue(line, []string{"标签", "文件类型"})
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, "http://"); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "https://"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.ReplaceAll(value, "#", " ")
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}

func qualityFromLine(line string) string {
	tokens := regexp.MustCompile(`(?i)\b(?:WEB[- ]?DL|WEB[- ]?4K|WEB|4K|8K|2160p|1080p|720p|BDISO|BluRay|REMUX|UHD|HDR10?|DV|SDR|DDP5?\.?1?|DTS-HD(?:\s+MA)?|HEVC|H\.?26[45]|AAC)\b`).FindAllString(line, -1)
	if len(tokens) == 0 {
		return ""
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		key := strings.ToLower(token)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, token)
	}
	return strings.Join(out, " ")
}

func mediaMetadataFromURL(typ string, raw string) mediaMetadata {
	switch typ {
	case "ed2k":
		return mediaMetadataFromED2K(raw)
	case "magnet":
		return mediaMetadataFromMagnet(raw)
	default:
		return mediaMetadata{}
	}
}

func mediaMetadataFromED2K(raw string) mediaMetadata {
	parts := strings.Split(raw, "|")
	if len(parts) < 5 {
		return mediaMetadata{}
	}
	name, err := url.QueryUnescape(parts[2])
	if err != nil {
		name = parts[2]
	}
	title := stripFileExtension(name)
	metadata := mediaMetadata{
		Title:   normalizeMediaTitle(title),
		Quality: qualityFromLine(title),
	}
	metadata.merge(sequenceMetadata(title))
	if metadata.Year == "" {
		metadata.Year = extractYear(title)
	}
	if metadata.Size == "" && parts[3] != "" {
		metadata.Size = formatBytesString(parts[3])
	}
	return metadata
}

func mediaMetadataFromMagnet(raw string) mediaMetadata {
	parsed, err := url.Parse(raw)
	if err != nil {
		return mediaMetadata{}
	}
	title := parsed.Query().Get("dn")
	if title == "" {
		return mediaMetadata{}
	}
	metadata := mediaMetadata{
		Title:   normalizeMediaTitle(stripFileExtension(title)),
		Quality: qualityFromLine(title),
	}
	metadata.merge(sequenceMetadata(title))
	if metadata.Year == "" {
		metadata.Year = extractYear(title)
	}
	return metadata
}

func stripFileExtension(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx == len(name)-1 {
		return name
	}
	ext := strings.ToLower(name[idx+1:])
	if regexp.MustCompile(`^[a-z0-9]{2,5}$`).MatchString(ext) {
		return name[:idx]
	}
	return name
}

func formatBytesString(raw string) string {
	var bytes uint64
	for _, r := range raw {
		if r < '0' || r > '9' {
			return ""
		}
		bytes = bytes*10 + uint64(r-'0')
	}
	if bytes == 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return raw + " B"
	}
	if value >= 10 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".") + " " + units[unit]
}

func extractFirstMatch(value string, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(value)
	if len(match) == 0 {
		return ""
	}
	if len(match) == 1 {
		return strings.TrimSpace(match[0])
	}
	return strings.TrimSpace(match[1])
}

func extractYear(value string) string {
	if year := extractFirstMatch(value, `(?i)年\s*代\s*((?:19|20)\d{2})`); year != "" {
		return year
	}
	return extractFirstMatch(value, `(?:19|20)\d{2}`)
}
