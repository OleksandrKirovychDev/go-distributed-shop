// Package auth issues and verifies RS256 access tokens. It is strictly a JWT
// library — it knows nothing about argon2, refresh tokens, or persistence.
//
// The user ID travels in the standard `sub` claim; the Claims.UserID field is
// an ergonomic mirror of it, populated by Verify and never serialised twice.
//
// The signing key may be ephemeral (GenerateEphemeralKey, dev only) or durable
// (LoadPrivateKeyFromPEM). Either way the issuer stamps a stable kid derived
// from the key (KeyID) and the public half is published as a JWKS document
// (JWKS) so an edge verifier — e.g. an API gateway — can validate tokens
// without sharing the private key.
package auth
