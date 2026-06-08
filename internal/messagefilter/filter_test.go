package messagefilter

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"tg-search/internal/model"
)

func TestFilterAppliesLinksIncludesAndExcludes(t *testing.T) {
	rules := fakeRules{byChannel: map[int64]model.WatchRule{
		1: {ChannelID: 1, Enabled: true, Includes: []string{"庆余年", "S01"}, Excludes: []string{"预告"}},
	}}
	filter := New(rules)

	result, err := filter.Apply(context.Background(), Request{
		ChannelID:      1,
		Text:           "庆余年 S01 https://pan.quark.cn/s/abc",
		RequireEnabled: true,
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !result.Keep || len(result.Links) != 1 || result.Links[0].Type != "quark" {
		t.Fatalf("result = %+v, want keep with quark link", result)
	}

	rejectedNoLink, _ := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "庆余年 S01", RequireEnabled: true})
	if rejectedNoLink.Keep || rejectedNoLink.Reason != ReasonNoLinks {
		t.Fatalf("no-link result = %+v, want ReasonNoLinks", rejectedNoLink)
	}

	rejectedInclude, _ := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "三体 https://pan.quark.cn/s/abc", RequireEnabled: true})
	if rejectedInclude.Keep || rejectedInclude.Reason != ReasonIncludeMiss {
		t.Fatalf("include result = %+v, want ReasonIncludeMiss", rejectedInclude)
	}

	rejectedExclude, _ := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "庆余年 预告 https://pan.quark.cn/s/abc", RequireEnabled: true})
	if rejectedExclude.Keep || rejectedExclude.Reason != ReasonExcludeHit {
		t.Fatalf("exclude result = %+v, want ReasonExcludeHit", rejectedExclude)
	}
}

func TestFilterHandlesMissingAndDisabledRules(t *testing.T) {
	rules := fakeRules{byChannel: map[int64]model.WatchRule{
		2: {ChannelID: 2, Enabled: false, Includes: []string{"庆余年"}},
	}}
	filter := New(rules)

	missing, err := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "anything", RequireRule: false})
	if err != nil {
		t.Fatalf("missing optional rule returned error: %v", err)
	}
	if !missing.Keep || missing.RuleApplied {
		t.Fatalf("missing optional result = %+v, want keep without rule", missing)
	}

	disabledRealtime, _ := filter.Apply(context.Background(), Request{ChannelID: 2, Text: "庆余年 https://pan.quark.cn/s/abc", RequireEnabled: true})
	if disabledRealtime.Keep || disabledRealtime.Reason != ReasonRuleDisabled {
		t.Fatalf("disabled realtime result = %+v, want ReasonRuleDisabled", disabledRealtime)
	}

	disabledHistory, _ := filter.Apply(context.Background(), Request{ChannelID: 2, Text: "庆余年 https://pan.quark.cn/s/abc", RequireEnabled: false})
	if !disabledHistory.Keep || !disabledHistory.RuleApplied {
		t.Fatalf("disabled history result = %+v, want keep because enabled ignored", disabledHistory)
	}
}

func TestFilterFallsBackToGlobalRuleWhenChannelRuleMissing(t *testing.T) {
	rules := fakeRules{
		byChannel: map[int64]model.WatchRule{},
		global:    &model.WatchRule{Enabled: true, Includes: []string{"庆余年"}, Excludes: []string{"预告"}},
	}
	filter := New(rules)

	kept, err := filter.Apply(context.Background(), Request{
		ChannelID:      7,
		Text:           "庆余年 https://pan.quark.cn/s/abc",
		RequireRule:    true,
		RequireEnabled: true,
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !kept.Keep || !kept.RuleApplied || len(kept.Links) != 1 {
		t.Fatalf("kept result = %+v, want global rule applied with link", kept)
	}

	rejected, err := filter.Apply(context.Background(), Request{
		ChannelID:      7,
		Text:           "三体 https://pan.quark.cn/s/abc",
		RequireRule:    true,
		RequireEnabled: true,
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if rejected.Keep || rejected.Reason != ReasonIncludeMiss {
		t.Fatalf("rejected result = %+v, want include miss from global rule", rejected)
	}
}

func TestFilterReturnsRuleLookupErrors(t *testing.T) {
	filter := New(errorRules{})
	_, err := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "x", RequireRule: true})
	if err == nil {
		t.Fatal("Apply returned nil error, want lookup error")
	}
}

type fakeRules struct {
	byChannel map[int64]model.WatchRule
	global    *model.WatchRule
}

func (f fakeRules) FindByChannelID(ctx context.Context, channelID int64) (model.WatchRule, error) {
	rule, ok := f.byChannel[channelID]
	if !ok {
		if f.global != nil {
			return *f.global, nil
		}
		return model.WatchRule{}, sql.ErrNoRows
	}
	return rule, nil
}

type errorRules struct{}

func (errorRules) FindByChannelID(ctx context.Context, channelID int64) (model.WatchRule, error) {
	return model.WatchRule{}, errors.New("database unavailable")
}
