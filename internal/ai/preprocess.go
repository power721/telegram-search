package ai

import (
	"regexp"
	"strings"
)

type PreProcessor struct{}

func NewPreProcessor() PreProcessor {
	return PreProcessor{}
}

var (
	yearPattern    = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
	qualityPattern = regexp.MustCompile(`(?i)\b(2160p|1080p|720p|4k|8k|web-dl|bluray|blu-?ray|hdrip|hdtv)\b`)
	episodePattern = regexp.MustCompile(`(?i)\bS(\d{1,2})E(\d{1,3})\b`)
	sizePattern    = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?\s*(?:GB|G|MB|M))\b`)
)

func (p PreProcessor) Extract(text string) map[string]any {
	hints := map[string]any{}
	text = strings.TrimSpace(text)
	if text == "" {
		return hints
	}
	if match := yearPattern.FindStringSubmatch(text); len(match) > 1 {
		hints["year"] = match[1]
	}
	if match := qualityPattern.FindStringSubmatch(text); len(match) > 1 {
		hints["quality"] = normalizeQuality(match[1])
	}
	if match := episodePattern.FindStringSubmatch(text); len(match) > 2 {
		hints["season"] = "S" + leftPad2(match[1])
		hints["episode"] = "E" + leftPad2(match[2])
	}
	if match := sizePattern.FindStringSubmatch(text); len(match) > 1 {
		hints["size"] = normalizeSize(match[1])
	}
	return hints
}

func normalizeQuality(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "4k":
		return "4K"
	case "8k":
		return "8K"
	default:
		return value
	}
}

func normalizeSize(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), " ", "")
}

func leftPad2(value string) string {
	if len(value) == 1 {
		return "0" + value
	}
	return value
}
