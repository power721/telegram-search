package telegram

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "china with spaces", input: "+86 13800138000", want: "+8613800138000"},
		{name: "china digits", input: "8613800138000", want: "+8613800138000"},
		{name: "us punctuation", input: "+1 (650) 253-0000", want: "+16502530000"},
		{name: "japan spaces", input: "+81 90 1234 5678", want: "+819012345678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePhone(tt.input)
			if err != nil {
				t.Fatalf("NormalizePhone(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizePhone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizePhoneInvalid(t *testing.T) {
	for _, input := range []string{"", "not-a-phone", "+86 123"} {
		t.Run(input, func(t *testing.T) {
			if got, err := NormalizePhone(input); err == nil {
				t.Fatalf("NormalizePhone(%q) = %q, want error", input, got)
			}
		})
	}
}
