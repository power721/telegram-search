package channel

import "fmt"

const (
	SyncProfileQuick  = "Quick"
	SyncProfileNormal = "Normal"
	SyncProfileDeep   = "Deep"
	SyncProfileFull   = "Full"
)

func ParseProfile(value string) (string, error) {
	switch value {
	case SyncProfileQuick, SyncProfileNormal, SyncProfileDeep, SyncProfileFull:
		return value, nil
	default:
		return "", fmt.Errorf("invalid sync profile %q", value)
	}
}

func ProfileLimit(value string) (int, error) {
	profile, err := ParseProfile(value)
	if err != nil {
		return 0, err
	}
	switch profile {
	case SyncProfileQuick:
		return 100, nil
	case SyncProfileNormal:
		return 1000, nil
	case SyncProfileDeep:
		return 10000, nil
	case SyncProfileFull:
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid sync profile %q", value)
	}
}
