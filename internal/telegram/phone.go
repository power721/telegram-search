package telegram

import (
	"errors"
	"strings"
	"unicode"

	"github.com/nyaruka/phonenumbers"
)

var ErrInvalidPhone = errors.New("invalid phone number")

func NormalizePhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "", ErrInvalidPhone
	}

	num, err := phonenumbers.Parse(phone, "")
	if err != nil && !strings.HasPrefix(phone, "+") {
		num, err = phonenumbers.Parse("+"+phoneDigits(phone), "")
	}
	if err != nil || !phonenumbers.IsValidNumber(num) {
		return "", ErrInvalidPhone
	}
	return phonenumbers.Format(num, phonenumbers.E164), nil
}

func phoneDigits(phone string) string {
	var builder strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
