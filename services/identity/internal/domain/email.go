package domain

import (
	"net/mail"
	"strings"
)

const maxEmailLength = 254

type Email string

// ParseEmail normalises (trim + lowercase) and validates a raw address. It
// rejects display-name forms ("Foo <a@b.c>") by requiring the parsed address to
// equal the normalised input.
func ParseEmail(raw string) (Email, error) {
	normalised := strings.ToLower(strings.TrimSpace(raw))
	if normalised == "" || len(normalised) > maxEmailLength {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(normalised)
	if err != nil || addr.Address != normalised {
		return "", ErrInvalidEmail
	}
	return Email(normalised), nil
}

func (e Email) String() string { return string(e) }
