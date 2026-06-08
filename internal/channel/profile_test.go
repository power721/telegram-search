package channel

import "testing"

func TestSyncProfileLimit(t *testing.T) {
	tests := []struct {
		profile string
		limit   int
	}{
		{SyncProfileQuick, 100},
		{SyncProfileNormal, 1000},
		{SyncProfileDeep, 10000},
		{SyncProfileFull, 0},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			limit, err := ProfileLimit(tt.profile)
			if err != nil {
				t.Fatalf("ProfileLimit returned error: %v", err)
			}
			if limit != tt.limit {
				t.Fatalf("ProfileLimit(%q) = %d, want %d", tt.profile, limit, tt.limit)
			}
		})
	}
}

func TestSyncProfileRejectsRawLimit(t *testing.T) {
	if _, err := ParseProfile("raw-1000"); err == nil {
		t.Fatal("ParseProfile accepted raw-1000, want error")
	}
}
