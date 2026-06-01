package auth_test

import (
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/online-shop/pkg/auth"
	"github.com/online-shop/pkg/errors"
)

const (
	testIssuer   = "identity"
	testAudience = "online-shop"
)

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := auth.GenerateEphemeralKey()
	if err != nil {
		t.Fatalf("GenerateEphemeralKey: %v", err)
	}
	return key
}

func TestIssueVerify_RoundTrip(t *testing.T) {
	t.Parallel()

	key := newKey(t)
	issuer := auth.NewTokenIssuer(key, testIssuer, testAudience, 15*time.Minute)
	verifier := auth.NewVerifier(&key.PublicKey, testIssuer, testAudience)

	token, exp, err := issuer.Issue("user-1", "a@b.c", []string{"customer", "admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if time.Until(exp) > 15*time.Minute || time.Until(exp) < 14*time.Minute {
		t.Fatalf("expiry not ~15m out: %v", time.Until(exp))
	}

	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1 (from sub)", claims.UserID)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("Subject = %q, want user-1", claims.Subject)
	}
	if claims.Email != "a@b.c" {
		t.Fatalf("Email = %q", claims.Email)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "customer" || claims.Roles[1] != "admin" {
		t.Fatalf("Roles = %v", claims.Roles)
	}
	if claims.Issuer != testIssuer {
		t.Fatalf("Issuer = %q", claims.Issuer)
	}
}

func TestVerify_RejectsAlgSubstitution(t *testing.T) {
	t.Parallel()

	// Classic downgrade attack: an attacker forges an HS256 token. WithValidMethods
	// must reject it before any signature check, regardless of the key.
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "attacker",
		Issuer:    testIssuer,
		Audience:  jwt.ClaimStrings{testAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	forged, err := hs.SignedString([]byte("guessed-secret"))
	if err != nil {
		t.Fatalf("sign HS256: %v", err)
	}

	key := newKey(t)
	verifier := auth.NewVerifier(&key.PublicKey, testIssuer, testAudience)
	if _, err := verifier.Verify(forged); !errors.IsKind(err, errors.KindUnauthorized) {
		t.Fatalf("expected Unauthorized for alg substitution, got %v", err)
	}
}

func TestVerify_RejectsWrongSigner(t *testing.T) {
	t.Parallel()

	signer := newKey(t)
	other := newKey(t)
	issuer := auth.NewTokenIssuer(signer, testIssuer, testAudience, time.Hour)
	verifier := auth.NewVerifier(&other.PublicKey, testIssuer, testAudience)

	token, _, err := issuer.Issue("user-1", "a@b.c", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Verify(token); !errors.IsKind(err, errors.KindUnauthorized) {
		t.Fatalf("expected Unauthorized for wrong signer, got %v", err)
	}
}

func TestVerify_RejectsExpired(t *testing.T) {
	t.Parallel()

	key := newKey(t)
	issuer := auth.NewTokenIssuer(key, testIssuer, testAudience, -time.Minute)
	verifier := auth.NewVerifier(&key.PublicKey, testIssuer, testAudience)

	token, _, err := issuer.Issue("user-1", "a@b.c", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Verify(token); !errors.IsKind(err, errors.KindUnauthorized) {
		t.Fatalf("expected Unauthorized for expired token, got %v", err)
	}
}

func TestVerify_RejectsWrongAudienceAndIssuer(t *testing.T) {
	t.Parallel()

	key := newKey(t)
	issuer := auth.NewTokenIssuer(key, testIssuer, testAudience, time.Hour)
	token, _, err := issuer.Issue("user-1", "a@b.c", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := auth.NewVerifier(&key.PublicKey, "other", testAudience).Verify(token); !errors.IsKind(err, errors.KindUnauthorized) {
		t.Fatalf("expected Unauthorized for wrong issuer, got %v", err)
	}
	if _, err := auth.NewVerifier(&key.PublicKey, testIssuer, "other").Verify(token); !errors.IsKind(err, errors.KindUnauthorized) {
		t.Fatalf("expected Unauthorized for wrong audience, got %v", err)
	}
}
