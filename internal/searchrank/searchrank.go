package searchrank

import "strings"

var titleMarkers = []struct {
	value string
	score int
}{
	{"合集", 120},
	{"系列", 110},
	{"最新", 100},
	{"complete", 90},
	{"完", 80},
	{"全", 70},
}

// TextMatchScore rewards matches in earlier fields more heavily. Pass fields
// from most-specific to least-specific, for example title, note, tags, content.
func TextMatchScore(query string, fields ...string) int {
	query = normalize(query)
	if query == "" {
		return 0
	}
	terms := queryTerms(query)
	score := 0
	for i, field := range fields {
		field = normalize(field)
		if field == "" {
			continue
		}
		weight := fieldWeight(i)
		switch {
		case field == query:
			score += weight + 600
		case strings.HasPrefix(field, query):
			score += weight + 360
		case strings.Contains(field, query):
			score += weight + 220
		}
		for _, term := range terms {
			if term != query && strings.Contains(field, term) {
				score += weight / 5
			}
		}
	}
	return score
}

func TitleMarkerScore(fields ...string) int {
	score := 0
	for _, field := range fields {
		field = normalize(field)
		if field == "" {
			continue
		}
		for _, marker := range titleMarkers {
			if strings.Contains(field, marker.value) {
				score += marker.score
			}
		}
	}
	return score
}

func MetadataScore(fields ...string) int {
	score := 0
	for _, field := range fields {
		if strings.TrimSpace(field) != "" {
			score += 18
		}
	}
	return score
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func queryTerms(query string) []string {
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return []string{query}
	}
	return parts
}

func fieldWeight(index int) int {
	switch index {
	case 0:
		return 500
	case 1:
		return 420
	case 2:
		return 320
	case 3:
		return 220
	default:
		return 120
	}
}
