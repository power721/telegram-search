package messagefilter

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"tg-provider/internal/link"
	"tg-provider/internal/model"
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
	ReasonIncludeMiss  Reason = "include_miss"
	ReasonExcludeHit   Reason = "exclude_hit"
)

func New(rules RuleStore) *Filter {
	return &Filter{rules: rules, extractor: link.NewExtractor()}
}

func (f *Filter) Apply(ctx context.Context, req Request) (Result, error) {
	if f == nil || f.rules == nil {
		return Result{Keep: true, Reason: ReasonNone}, nil
	}
	rule, err := f.rules.FindByChannelID(ctx, req.ChannelID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if req.RequireRule {
				return Result{Keep: false, Reason: ReasonNoRule}, nil
			}
			return Result{Keep: true, Reason: ReasonNoRule}, nil
		}
		return Result{}, err
	}
	if req.RequireEnabled && !rule.Enabled {
		return Result{Keep: false, RuleApplied: true, Reason: ReasonRuleDisabled}, nil
	}
	extractor := f.extractor
	if extractor == nil {
		extractor = link.NewExtractor()
	}
	links := extractor.Extract(req.Text)
	if len(links) == 0 {
		return Result{Keep: false, RuleApplied: true, Reason: ReasonNoLinks}, nil
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
