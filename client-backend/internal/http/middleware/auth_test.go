package middleware

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuth0VerifierAcceptsValidRS256Token(t *testing.T) {
	key := newTestRSAKey(t)
	jwks := newTestJWKSServer(t, key, "kid-valid")
	defer jwks.Close()

	verifier := NewAuth0Verifier(AuthConfig{
		Issuer:   "https://exos-test.us.auth0.com/",
		Audience: "https://api.exos.tech",
		JWKSURL:  jwks.URL,
	})

	token := signTestToken(t, key, "kid-valid", map[string]any{
		"iss":   "https://exos-test.us.auth0.com/",
		"sub":   "auth0|user-123",
		"aud":   []string{"https://api.exos.tech"},
		"email": "ops@exos.tech",
		"name":  "Ops User",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Add(-time.Minute).Unix(),
	})

	identity, err := verifier.IdentityFromBearer(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("IdentityFromBearer returned error: %v", err)
	}
	if identity.Auth0Sub != "auth0|user-123" || identity.Email != "ops@exos.tech" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestAuth0VerifierRejectsWrongAudience(t *testing.T) {
	key := newTestRSAKey(t)
	jwks := newTestJWKSServer(t, key, "kid-valid")
	defer jwks.Close()

	verifier := NewAuth0Verifier(AuthConfig{
		Issuer:   "https://exos-test.us.auth0.com/",
		Audience: "https://api.exos.tech",
		JWKSURL:  jwks.URL,
	})

	token := signTestToken(t, key, "kid-valid", map[string]any{
		"iss": "https://exos-test.us.auth0.com/",
		"sub": "auth0|user-123",
		"aud": []string{"https://some-other-api.example"},
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	if _, err := verifier.IdentityFromBearer(context.Background(), "Bearer "+token); err == nil {
		t.Fatal("expected wrong audience token to be rejected")
	}
}

func TestAuth0VerifierUsesClientIDWhenAudienceIsBlank(t *testing.T) {
	key := newTestRSAKey(t)
	jwks := newTestJWKSServer(t, key, "kid-valid")
	defer jwks.Close()

	verifier := NewAuth0Verifier(AuthConfig{
		Issuer:   "https://exos-test.us.auth0.com/",
		ClientID: "spa-client-id",
		JWKSURL:  jwks.URL,
	})

	token := signTestToken(t, key, "kid-valid", map[string]any{
		"iss":   "https://exos-test.us.auth0.com/",
		"sub":   "auth0|user-123",
		"aud":   "spa-client-id",
		"email": "ops@exos.tech",
		"name":  "Ops User",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	identity, err := verifier.IdentityFromBearer(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("IdentityFromBearer returned error: %v", err)
	}
	if identity.Auth0Sub != "auth0|user-123" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func newTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func newTestJWKSServer(t *testing.T, key *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	payload := map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": kid,
				"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(bigEndianExponent(key.PublicKey.E)),
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("encode jwks: %v", err)
		}
	}))
}

func signTestToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid}
	signingInput := encodeJWTJSON(t, header) + "." + encodeJWTJSON(t, claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeJWTJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal jwt json: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func bigEndianExponent(exponent int) []byte {
	if exponent == 0 {
		return nil
	}
	var out []byte
	for exponent > 0 {
		out = append([]byte{byte(exponent & 0xff)}, out...)
		exponent >>= 8
	}
	return out
}
