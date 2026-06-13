package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func decodeEnhancementResponse(content string) (EnhancementResponse, error) {
	content = extractJSONObject(content)
	out, err := unmarshalEnhancementResponse(content)
	if err == nil {
		return out, nil
	}
	repaired := RepairJSON(content)
	out, repairErr := unmarshalEnhancementResponse(repaired)
	if repairErr != nil {
		return EnhancementResponse{}, fmt.Errorf("decode media metadata json: %w", err)
	}
	return out, nil
}

func unmarshalEnhancementResponse(content string) (EnhancementResponse, error) {
	var out EnhancementResponse
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return EnhancementResponse{}, err
	}
	if out.Items == nil {
		out.Items = []EnhancementItem{}
	}
	return out, nil
}

var trailingCommaPattern = regexp.MustCompile(`,\s*([}\]])`)

func RepairJSON(raw string) string {
	raw = extractJSONObject(raw)
	raw = trailingCommaPattern.ReplaceAllString(raw, "$1")
	raw = balanceJSON(raw)
	return raw
}

func balanceJSON(raw string) string {
	openObjects := strings.Count(raw, "{")
	closeObjects := strings.Count(raw, "}")
	openArrays := strings.Count(raw, "[")
	closeArrays := strings.Count(raw, "]")
	for closeArrays < openArrays {
		raw += "]"
		closeArrays++
	}
	for closeObjects < openObjects {
		raw += "}"
		closeObjects++
	}
	return raw
}

func ValidateEnhancementResponse(resp EnhancementResponse, input EnhancementRequest) error {
	if len(input.Links) > 0 && len(resp.Items) == 0 {
		return errors.New("ai media metadata response has empty items")
	}
	linkIDs := map[int64]struct{}{}
	linkURLs := map[string]struct{}{}
	for _, link := range input.Links {
		if link.LinkID != 0 {
			linkIDs[link.LinkID] = struct{}{}
		}
		if strings.TrimSpace(link.URL) != "" {
			linkURLs[strings.TrimSpace(link.URL)] = struct{}{}
		}
	}
	for i, item := range resp.Items {
		if item.LinkID == 0 && strings.TrimSpace(item.URL) == "" {
			return fmt.Errorf("ai media metadata item %d missing identifier", i)
		}
		if len(input.Links) > 0 && !knownEnhancementItem(item, linkIDs, linkURLs) {
			return fmt.Errorf("ai media metadata item %d references unknown link", i)
		}
	}
	return nil
}

func knownEnhancementItem(item EnhancementItem, linkIDs map[int64]struct{}, linkURLs map[string]struct{}) bool {
	if item.LinkID != 0 {
		if _, ok := linkIDs[item.LinkID]; ok {
			return true
		}
	}
	if strings.TrimSpace(item.URL) != "" {
		if _, ok := linkURLs[strings.TrimSpace(item.URL)]; ok {
			return true
		}
	}
	return false
}
