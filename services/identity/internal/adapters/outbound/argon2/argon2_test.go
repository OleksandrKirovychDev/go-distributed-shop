package argon2_test

import (
	"context"
	"strings"
	"testing"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/adapters/outbound/argon2"
	"github.com/online-shop/services/identity/internal/domain"
)

// fastParams keeps the KDF cheap for tests while still exercising the full PHC
// encode/decode round-trip.
func fastParams() argon2.Params {
	return argon2.Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
}

func TestPasswordHasher_HashThenVerify(t *testing.T) {
	t.Parallel()

	h := argon2.NewPasswordHasher(fastParams())
	ctx := context.Background()

	hash, err := h.Hash(ctx, "hunter2!")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(string(hash), "$argon2id$") {
		t.Fatalf("hash is not PHC-encoded: %q", hash)
	}

	ok, err := h.Verify(ctx, "hunter2!", hash)
	if err != nil || !ok {
		t.Fatalf("Verify(correct) = (%v, %v), want (true, nil)", ok, err)
	}

	ok, err = h.Verify(ctx, "wrong-password", hash)
	if err != nil {
		t.Fatalf("Verify(wrong) error: %v", err)
	}
	if ok {
		t.Fatal("Verify(wrong) = true, want false")
	}
}

func TestPasswordHasher_SaltsAreUnique(t *testing.T) {
	t.Parallel()

	h := argon2.NewPasswordHasher(fastParams())
	ctx := context.Background()

	a, _ := h.Hash(ctx, "hunter2!")
	b, _ := h.Hash(ctx, "hunter2!")
	if a == b {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}

func TestPasswordHasher_VerifyRejectsMalformedHash(t *testing.T) {
	t.Parallel()

	h := argon2.NewPasswordHasher(fastParams())
	ok, err := h.Verify(context.Background(), "hunter2!", domain.PasswordHash("not-a-phc-hash"))
	if ok {
		t.Fatal("malformed hash must not verify")
	}
	if !errors.IsKind(err, errors.KindInternal) {
		t.Fatalf("error = %v, want Internal", err)
	}
}

func TestRefreshTokenGenerator(t *testing.T) {
	t.Parallel()

	g := argon2.NewRefreshTokenGenerator()

	token, hash, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if token == "" || hash == "" {
		t.Fatalf("empty token/hash: %q/%q", token, hash)
	}
	if hash == token {
		t.Fatal("stored hash must not equal the plaintext token")
	}
	if g.Hash(token) != hash {
		t.Fatal("Hash(token) must equal the hash returned by Generate")
	}

	other, _, _ := g.Generate()
	if other == token {
		t.Fatal("two generated tokens must differ")
	}
}
