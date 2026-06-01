package domain

import "unicode/utf8"

const (
	minPasswordLength = 8
	maxPasswordLength = 1024
)

type PlainPassword string

func (p PlainPassword) Validate() error {
	n := utf8.RuneCountInString(string(p))
	if n < minPasswordLength || n > maxPasswordLength {
		return ErrWeakPassword
	}
	return nil
}

// String redacts so a PlainPassword never lands in a log line or error via the
// default %s/%v formatting. The KDF adapter reads the secret with []byte(p).
func (p PlainPassword) String() string { return "[REDACTED]" }

type PasswordHash string

func (h PasswordHash) String() string { return string(h) }
