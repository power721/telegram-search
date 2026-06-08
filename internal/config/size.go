package config

import (
	"fmt"
	"strconv"
	"strings"
)

type Size int64

func ParseSize(input string) (Size, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, fmt.Errorf("size is required")
	}
	upper := strings.ToUpper(value)
	multiplier := int64(1)
	for _, suffix := range []struct {
		text string
		mul  int64
	}{
		{"GB", 1000 * 1000 * 1000},
		{"MB", 1000 * 1000},
		{"KB", 1000},
		{"B", 1},
	} {
		if strings.HasSuffix(upper, suffix.text) {
			multiplier = suffix.mul
			value = strings.TrimSpace(value[:len(value)-len(suffix.text)])
			break
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse size %q: %w", input, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("size must be non-negative")
	}
	return Size(n * multiplier), nil
}

func (s *Size) UnmarshalYAML(unmarshal func(any) error) error {
	var raw any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch value := raw.(type) {
	case int:
		*s = Size(value)
		return nil
	case int64:
		*s = Size(value)
		return nil
	case string:
		parsed, err := ParseSize(value)
		if err != nil {
			return err
		}
		*s = parsed
		return nil
	default:
		return fmt.Errorf("unsupported size value %T", raw)
	}
}
