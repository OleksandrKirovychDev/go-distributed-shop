package argon2

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/online-shop/services/identity/internal/app/ports"
)

const refreshTokenBytes = 32 // 256 bits of entropy

// RefreshTokenGenerator mints opaque refresh tokens. The token is full-entropy
// CSPRNG output, so a fast SHA-256 (not a slow KDF) is the right at-rest digest:
// there is nothing to brute-force, and it keeps lookup an O(1) indexed equality.
type RefreshTokenGenerator struct{}

func NewRefreshTokenGenerator() *RefreshTokenGenerator { return &RefreshTokenGenerator{} }

func (g *RefreshTokenGenerator) Generate() (token, hash string, err error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("refresh token: read random: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, g.Hash(token), nil
}

func (g *RefreshTokenGenerator) Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

var _ ports.RefreshTokenGenerator = (*RefreshTokenGenerator)(nil)
