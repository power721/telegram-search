package config

import "testing"

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  Size
	}{
		{"10GB", Size(10 * 1000 * 1000 * 1000)},
		{"20gb", Size(20 * 1000 * 1000 * 1000)},
		{"512MB", Size(512 * 1000 * 1000)},
		{"1024", Size(1024)},
	}
	for _, tt := range tests {
		got, err := ParseSize(tt.input)
		if err != nil {
			t.Fatalf("ParseSize(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseSizeRejectsInvalidValue(t *testing.T) {
	for _, input := range []string{"", "abc", "-1GB", "10XB"} {
		if _, err := ParseSize(input); err == nil {
			t.Fatalf("ParseSize(%q) returned nil error", input)
		}
	}
}
