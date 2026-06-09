package messagefilter

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"

	"tg-search/internal/link"
	"tg-search/internal/model"
)

type RuleStore interface {
	FindByChannelID(context.Context, int64) (model.WatchRule, error)
}

type Filter struct {
	rules     RuleStore
	extractor *link.Extractor
}

type Request struct {
	ChannelID      int64
	Text           string
	MessageType    string
	Files          []model.File
	RequireRule    bool
	RequireEnabled bool
}

type Result struct {
	Keep        bool
	RuleApplied bool
	Links       []model.Link
	Reason      Reason
}

type Reason string

const (
	ReasonNone         Reason = ""
	ReasonNoRule       Reason = "no_rule"
	ReasonRuleDisabled Reason = "rule_disabled"
	ReasonNoLinks      Reason = "no_links"
	ReasonLinkTypeMiss Reason = "link_type_miss"
	ReasonTypeMiss     Reason = "message_type_miss"
	ReasonIncludeMiss  Reason = "include_miss"
	ReasonExcludeHit   Reason = "exclude_hit"
)

var defaultIgnoredLinkPatterns = []string{"t.me", "toapp.mypikpak.com", "telegra.ph", "www.themoviedb.org"}

func DefaultIgnoredLinkPatterns() []string {
	return append([]string(nil), defaultIgnoredLinkPatterns...)
}

func New(rules RuleStore) *Filter {
	return &Filter{rules: rules, extractor: link.NewExtractor()}
}

func (f *Filter) Apply(ctx context.Context, req Request) (Result, error) {
	if f == nil || f.rules == nil {
		return Result{Keep: true, Reason: ReasonNone}, nil
	}
	extractor := f.extractor
	if extractor == nil {
		extractor = link.NewExtractor()
	}
	rawLinks := extractor.Extract(req.Text)
	rule, err := f.rules.FindByChannelID(ctx, req.ChannelID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			links := filterLinksByIgnoredPatterns(rawLinks, defaultIgnoredLinkPatterns)
			if req.RequireRule {
				return Result{Keep: false, Links: links, Reason: ReasonNoRule}, nil
			}
			return Result{Keep: true, Links: links, Reason: ReasonNoRule}, nil
		}
		return Result{}, err
	}
	if req.RequireEnabled && !rule.Enabled {
		return Result{Keep: false, RuleApplied: true, Reason: ReasonRuleDisabled}, nil
	}
	rawLinks = filterLinksByIgnoredPatterns(rawLinks, ignoredPatternsOrDefault(rule.IgnoredLinkPatterns))
	links := filterLinksByType(rawLinks, rule.LinkTypes)
	keepByType, reason := keepByMessageType(req, rule.MessageTypes, links, len(rawLinks) > 0)
	if !keepByType {
		return Result{Keep: false, RuleApplied: true, Links: links, Reason: reason}, nil
	}
	text := strings.ToLower(req.Text)
	if len(rule.Includes) > 0 && !containsAny(text, rule.Includes) {
		return Result{Keep: false, RuleApplied: true, Links: links, Reason: ReasonIncludeMiss}, nil
	}
	if containsAny(text, rule.Excludes) {
		return Result{Keep: false, RuleApplied: true, Links: links, Reason: ReasonExcludeHit}, nil
	}
	return Result{Keep: true, RuleApplied: true, Links: links, Reason: ReasonNone}, nil
}

func ignoredPatternsOrDefault(patterns []string) []string {
	if patterns == nil {
		return defaultIgnoredLinkPatterns
	}
	return patterns
}

func containsAny(lowerText string, terms []string) bool {
	for _, term := range terms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		if strings.Contains(lowerText, normalized) {
			return true
		}
	}
	return false
}

func filterLinksByType(links []model.Link, allowed []string) []model.Link {
	if len(links) == 0 || len(allowed) == 0 {
		return links
	}
	allowedSet := normalizedSet(allowed)
	out := make([]model.Link, 0, len(links))
	for _, item := range links {
		category := strings.ToLower(strings.TrimSpace(item.Category))
		if category == "" {
			category = linkCategory(item.Type)
		}
		if allowedSet[category] || allowedSet[strings.ToLower(strings.TrimSpace(item.Type))] {
			out = append(out, item)
			continue
		}
		if category != "cloud_drive" && category != "magnet" && category != "ed2k" && category != "http" && allowedSet["other"] {
			out = append(out, item)
		}
	}
	return out
}

func filterLinksByIgnoredPatterns(links []model.Link, patterns []string) []model.Link {
	if len(links) == 0 || len(patterns) == 0 {
		return links
	}
	out := make([]model.Link, 0, len(links))
	for _, item := range links {
		if ignoredLinkURL(item.URL, patterns) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func ignoredLinkURL(raw string, patterns []string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	lowerURL := strings.ToLower(raw)
	for _, pattern := range patterns {
		normalized := strings.ToLower(strings.TrimSpace(pattern))
		if normalized == "" {
			continue
		}
		if strings.HasPrefix(normalized, "*.") {
			base := strings.TrimPrefix(normalized, "*.")
			if host != base && strings.HasSuffix(host, "."+base) {
				return true
			}
			continue
		}
		if strings.Contains(normalized, "://") || strings.Contains(normalized, "/") {
			if strings.HasPrefix(lowerURL, normalized) {
				return true
			}
			continue
		}
		if host == normalized {
			return true
		}
	}
	return false
}

func keepByMessageType(req Request, messageTypes []string, links []model.Link, hasRawLinks bool) (bool, Reason) {
	if len(messageTypes) == 0 {
		if hasRawLinks && len(links) == 0 {
			return false, ReasonLinkTypeMiss
		}
		if len(links) == 0 {
			return false, ReasonNoLinks
		}
		return true, ReasonNone
	}
	allowed := normalizedSet(messageTypes)
	if len(links) > 0 {
		if allowed["link"] {
			return true, ReasonNone
		}
		return false, ReasonTypeMiss
	}
	if hasRawLinks {
		if keepByNonLinkMessageType(req, allowed) {
			return true, ReasonNone
		}
		return false, ReasonLinkTypeMiss
	}
	if keepByNonLinkMessageType(req, allowed) {
		return true, ReasonNone
	}
	if len(req.Files) > 0 || strings.TrimSpace(req.MessageType) != "" {
		return false, ReasonTypeMiss
	}
	return false, ReasonNoLinks
}

func keepByNonLinkMessageType(req Request, allowed map[string]bool) bool {
	messageType := strings.ToLower(strings.TrimSpace(req.MessageType))
	switch {
	case messageType == "photo" && (allowed["image"] || allowed["photo"] || allowed["file"]):
		return true
	case messageType == "video" && (allowed["video"] || allowed["file"]):
		return true
	case messageType == "audio" && (allowed["audio"] || allowed["file"]):
		return true
	case messageType == "file" && allowed["file"]:
		return true
	case messageType == "text" && allowed["text"]:
		return true
	case messageType != "" && allowed[messageType]:
		return true
	case len(req.Files) > 0 && allowed["file"]:
		return true
	case strings.TrimSpace(req.Text) != "" && allowed["text"]:
		return true
	default:
		return false
	}
}

func normalizedSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized != "" {
			out[normalized] = true
		}
	}
	return out
}

func linkCategory(typ string) string {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "magnet":
		return "magnet"
	case "ed2k":
		return "ed2k"
	case "url":
		return "http"
	case "":
		return ""
	default:
		return "cloud_drive"
	}
}
