package middleware

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const userKey = "exos.user"

var (
	errAuth0NotConfigured = errors.New("auth0 is not configured")
	errInvalidToken       = errors.New("invalid auth0 token")
)

type AuthConfig struct {
	Issuer                  string
	Audience                string
	ClientID                string
	JWKSURL                 string
	AllowInsecureDevHeaders bool
	HTTPClient              *http.Client
}

type Auth0Verifier struct {
	cfg       AuthConfig
	client    *http.Client
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Sub       string          `json:"sub"`
	Email     string          `json:"email"`
	Name      string          `json:"name"`
	Issuer    string          `json:"iss"`
	Audience  json.RawMessage `json:"aud"`
	Expires   int64           `json:"exp"`
	NotBefore int64           `json:"nbf"`
	IssuedAt  int64           `json:"iat"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func RequireAuth(svc *services.Services, configs ...AuthConfig) gin.HandlerFunc {
	cfg := AuthConfig{AllowInsecureDevHeaders: true}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	verifier := NewAuth0Verifier(cfg)

	return func(c *gin.Context) {
		identity, err := verifier.IdentityFromRequest(c.Request)
		if err != nil {
			log.Printf("auth failed: path=%s method=%s err=%v", c.Request.URL.Path, c.Request.Method, err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		user, err := svc.UpsertUser(c.Request.Context(), identity)
		if err != nil {
			log.Printf("user resolution failed: path=%s method=%s auth0_sub=%q err=%v", c.Request.URL.Path, c.Request.Method, identity.Auth0Sub, err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user could not be resolved"})
			return
		}
		c.Set(userKey, user)
		c.Next()
	}
}

func NewAuth0Verifier(cfg AuthConfig) *Auth0Verifier {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	cfg.Issuer = strings.TrimSpace(cfg.Issuer)
	cfg.Audience = strings.TrimSpace(cfg.Audience)
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.JWKSURL = strings.TrimSpace(cfg.JWKSURL)

	return &Auth0Verifier{
		cfg:    cfg,
		client: cfg.HTTPClient,
		keys:   make(map[string]*rsa.PublicKey),
	}
}

func (v *Auth0Verifier) IdentityFromRequest(r *http.Request) (store.AuthIdentity, error) {
	if v.cfg.AllowInsecureDevHeaders {
		if identity := identityFromHeaders(r); identity.Auth0Sub != "" {
			return identity, nil
		}
	}
	return v.IdentityFromBearer(r.Context(), r.Header.Get("Authorization"))
}

func (v *Auth0Verifier) IdentityFromBearer(ctx context.Context, header string) (store.AuthIdentity, error) {
	if !strings.HasPrefix(header, "Bearer ") {
		return store.AuthIdentity{}, fmt.Errorf("%w: missing bearer header", errInvalidToken)
	}
	if !v.configured() {
		return store.AuthIdentity{}, errAuth0NotConfigured
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return store.AuthIdentity{}, fmt.Errorf("%w: token does not have three jwt parts", errInvalidToken)
	}

	var headerClaims jwtHeader
	if err := decodeJWTPart(parts[0], &headerClaims); err != nil {
		return store.AuthIdentity{}, fmt.Errorf("%w: decode header: %v", errInvalidToken, err)
	}
	if headerClaims.Alg != "RS256" || headerClaims.Kid == "" {
		return store.AuthIdentity{}, fmt.Errorf("%w: unsupported alg or missing kid", errInvalidToken)
	}

	var claims jwtClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return store.AuthIdentity{}, fmt.Errorf("%w: decode claims: %v", errInvalidToken, err)
	}
	if err := v.validateClaims(claims, time.Now()); err != nil {
		return store.AuthIdentity{}, err
	}

	key, err := v.publicKey(ctx, headerClaims.Kid)
	if err != nil {
		return store.AuthIdentity{}, err
	}
	if err := verifyRS256(parts[0]+"."+parts[1], parts[2], key); err != nil {
		return store.AuthIdentity{}, err
	}

	return store.AuthIdentity{Auth0Sub: claims.Sub, Email: claims.Email, Name: claims.Name}, nil
}

func (v *Auth0Verifier) configured() bool {
	return v.cfg.Issuer != "" && v.audience() != "" && v.cfg.JWKSURL != ""
}

func (v *Auth0Verifier) validateClaims(claims jwtClaims, now time.Time) error {
	if claims.Sub == "" {
		return fmt.Errorf("%w: missing subject", errInvalidToken)
	}
	if claims.Issuer != v.cfg.Issuer {
		return fmt.Errorf("%w: issuer mismatch got=%q want=%q", errInvalidToken, claims.Issuer, v.cfg.Issuer)
	}
	if claims.Expires == 0 || now.After(time.Unix(claims.Expires, 0)) {
		return fmt.Errorf("%w: token expired or missing exp", errInvalidToken)
	}
	if claims.NotBefore != 0 && now.Before(time.Unix(claims.NotBefore, 0)) {
		return fmt.Errorf("%w: token not yet valid", errInvalidToken)
	}
	if !audienceContains(claims.Audience, v.audience()) {
		return fmt.Errorf("%w: audience mismatch want=%q", errInvalidToken, v.audience())
	}
	return nil
}

func (v *Auth0Verifier) audience() string {
	if v.cfg.Audience != "" {
		return v.cfg.Audience
	}
	return v.cfg.ClientID
}

func (v *Auth0Verifier) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	cacheFresh := time.Since(v.fetchedAt) < 10*time.Minute
	v.mu.RUnlock()
	if ok && cacheFresh {
		return key, nil
	}

	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, errInvalidToken
	}
	return key, nil
}

func (v *Auth0Verifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("fetch auth0 jwks: status %d", resp.StatusCode)
	}

	var payload jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, item := range payload.Keys {
		key, err := item.publicKey()
		if err != nil {
			continue
		}
		keys[item.Kid] = key
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func (j jwk) publicKey() (*rsa.PublicKey, error) {
	if j.Kid == "" || j.Kty != "RSA" || j.N == "" || j.E == "" {
		return nil, errInvalidToken
	}
	modulusBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, err
	}
	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, errInvalidToken
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulusBytes), E: exponent}, nil
}

func RequireOrganizationPermission(svc *services.Services, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		organizationID, err := uuid.Parse(c.Param("organizationId"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
			return
		}
		if err := svc.RequirePermission(c.Request.Context(), user.ID, organizationID, permission); err != nil {
			if CanViewOrganizationAsPlatformAdmin(c, svc) {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing organization permission", "permission": permission})
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (domain.User, bool) {
	value, ok := c.Get(userKey)
	if !ok {
		return domain.User{}, false
	}
	user, ok := value.(domain.User)
	return user, ok
}

func identityFromHeaders(r *http.Request) store.AuthIdentity {
	return store.AuthIdentity{
		Auth0Sub: strings.TrimSpace(r.Header.Get("X-Auth0-Sub")),
		Email:    strings.TrimSpace(r.Header.Get("X-User-Email")),
		Name:     strings.TrimSpace(r.Header.Get("X-User-Name")),
	}
}

func decodeJWTPart(part string, out any) error {
	payload, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, out)
}

func audienceContains(raw json.RawMessage, audience string) bool {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single == audience
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		for _, item := range many {
			if item == audience {
				return true
			}
		}
	}
	return false
}

func verifyRS256(signingInput, encodedSignature string, key *rsa.PublicKey) error {
	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return errInvalidToken
	}
	digest := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return errInvalidToken
	}
	return nil
}
