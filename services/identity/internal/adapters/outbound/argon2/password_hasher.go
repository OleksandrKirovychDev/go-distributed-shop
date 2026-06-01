// Package argon2 implements the credential-crypto outbound ports: Argon2id
// password hashing (PHC-encoded, params from config) and opaque refresh-token
// generation (CSPRNG + SHA-256). Both are config + OS randomness, which is why
// they are adapters rather than pure domain.
package argon2

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type Params struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultParams() Params {
	return Params{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLength: 16, KeyLength: 32}
}

type PasswordHasher struct {
	params Params
}

func NewPasswordHasher(params Params) *PasswordHasher {
	return &PasswordHasher{params: params}
}

func (h *PasswordHasher) Hash(_ context.Context, plain domain.PlainPassword) (domain.PasswordHash, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain), salt, h.params.Iterations, h.params.Memory, h.params.Parallelism, h.params.KeyLength)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.Memory, h.params.Iterations, h.params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return domain.PasswordHash(encoded), nil
}

func (h *PasswordHasher) Verify(_ context.Context, plain domain.PlainPassword, hash domain.PasswordHash) (bool, error) {
	decoded, err := decodeHash(string(hash))
	if err != nil {
		return false, err
	}
	keyLen := uint32(len(decoded.key)) //nolint:gosec // decoded argon2 key is a small fixed length; no overflow
	got := argon2.IDKey([]byte(plain), decoded.salt, decoded.params.Iterations, decoded.params.Memory, decoded.params.Parallelism, keyLen)
	return subtle.ConstantTimeCompare(got, decoded.key) == 1, nil
}

type decodedHash struct {
	params Params
	salt   []byte
	key    []byte
}

func decodeHash(encoded string) (decodedHash, error) {
	fail := func(msg string, cause error) (decodedHash, error) {
		return decodedHash{}, errors.NewInternal("identity.bad_password_hash", msg, cause)
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fail("unrecognised password hash format", nil)
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return fail("unsupported argon2 version", err)
	}

	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return fail("malformed argon2 parameters", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fail("malformed salt", err)
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fail("malformed key", err)
	}

	return decodedHash{params: p, salt: salt, key: key}, nil
}

var _ ports.PasswordHasher = (*PasswordHasher)(nil)
