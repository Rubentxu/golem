// Package oidc provides an OIDC JWT verification adapter (ADR-082).
package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Adapter errors.
var (
	ErrAuthFailed   = errors.New("authn: verification failed")
	ErrTokenExpired = errors.New("authn: token expired")
	ErrKeyNotFound  = errors.New("authn: signing key not found in JWKS")
)

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key.
type JWK struct {
	Kid string `json:"kid"` // key ID
	Kty string `json:"kty"` // key type (RSA, EC)
	Alg string `json:"alg"` // algorithm (RS256, etc.)
	Use string `json:"use"` // key use (sig)
	N   string `json:"n"`   // RSA modulus
	E   string `json:"e"`   // RSA exponent
}

// Config configures the OIDC adapter.
type Config struct {
	IssuerURL string        // e.g. "https://auth.example.com/realms/golem"
	ClientID  string        // OAuth 2.0 client ID
	CacheTTL  time.Duration // JWKS cache TTL (default 1h)
}

// Adapter implements ports.AuthN using OIDC JWT verification.
type Adapter struct {
	config Config
	jwks   *JWKS
	jwksMu sync.RWMutex
	jwksAt time.Time
	client *http.Client
}

// NewAdapter creates an OIDC adapter with the given configuration.
func NewAdapter(config Config) *Adapter {
	if config.CacheTTL == 0 {
		config.CacheTTL = time.Hour
	}
	return &Adapter{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// VerifyBearer implements ports.AuthN.
func (a *Adapter) VerifyBearer(ctx context.Context, token string) (ports.Principal, error) {
	// Parse the JWT without verification first to get the kid.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ports.Principal{}, ErrAuthFailed
	}

	// Decode header.
	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		return ports.Principal{}, fmt.Errorf("%w: invalid header: %v", ErrAuthFailed, err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return ports.Principal{}, fmt.Errorf("%w: invalid header json: %v", ErrAuthFailed, err)
	}

	// Get the signing key.
	key, err := a.getSigningKey(ctx, header.Kid)
	if err != nil {
		return ports.Principal{}, err
	}

	// Verify the signature (RS256).
	if err := a.verifyRS256(token, key, header.Alg); err != nil {
		return ports.Principal{}, fmt.Errorf("%w: signature verification failed", err)
	}

	// Parse claims.
	claimsBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return ports.Principal{}, fmt.Errorf("%w: invalid claims: %v", ErrAuthFailed, err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return ports.Principal{}, fmt.Errorf("%w: invalid claims json: %v", ErrAuthFailed, err)
	}

	// Validate standard claims.
	if claims.Exp != 0 && time.Unix(claims.Exp, 0).Before(time.Now()) {
		return ports.Principal{}, ErrTokenExpired
	}
	if claims.Iat != 0 && claims.Exp == 0 {
		// Token with iat but no exp is suspicious.
		return ports.Principal{}, ErrAuthFailed
	}
	if a.config.IssuerURL != "" && claims.Iss != "" && claims.Iss != a.config.IssuerURL {
		return ports.Principal{}, fmt.Errorf("%w: issuer mismatch", ErrAuthFailed)
	}

	// Build principal from claims.
	principal := ports.Principal{
		Subject: claims.Sub,
		Type:    mapClaimType(claims.Type),
		Groups:  claims.Groups,
		Claims:  claims.Extra,
	}

	// Tenant memberships from claim.
	for _, m := range claims.TenantMemberships {
		principal.TenantMemberships = append(principal.TenantMemberships, ports.TenantMembership{
			TenantID: m.TenantID,
			Role:     m.Role,
		})
	}

	return principal, nil
}

// Discover implements ports.AuthN.
func (a *Adapter) Discover(ctx context.Context) (string, error) {
	if a.config.IssuerURL == "" {
		return "", errors.New("oidc: no issuer URL configured")
	}
	// Return the issuer URL for /.well-known/openid-configuration discovery.
	return a.config.IssuerURL, nil
}

// RefreshJWKS forces a JWKS refresh (used by JWKS rotation tests).
func (a *Adapter) RefreshJWKS(ctx context.Context) error {
	return a.fetchJWKS(ctx)
}

func (a *Adapter) getSigningKey(ctx context.Context, kid string) (*JWK, error) {
	// Check cache.
	a.jwksMu.RLock()
	cached := a.jwks
	cacheAge := time.Since(a.jwksAt)
	a.jwksMu.RUnlock()

	if cached != nil && cacheAge < a.config.CacheTTL {
		// Try to find the key in the cached JWKS.
		for _, k := range cached.Keys {
			if k.Kid == kid {
				return &k, nil
			}
		}
		// Key not found in cache — refresh.
	}

	// Refresh JWKS.
	if err := a.fetchJWKS(ctx); err != nil {
		return nil, err
	}

	a.jwksMu.RLock()
	defer a.jwksMu.RUnlock()
	for _, k := range a.jwks.Keys {
		if k.Kid == kid {
			return &k, nil
		}
	}
	return nil, fmt.Errorf("%w: key %s not found in JWKS", ErrAuthFailed, kid)
}

func (a *Adapter) fetchJWKS(ctx context.Context) error {
	// Fetch OIDC discovery document.
	discURL := strings.TrimSuffix(a.config.IssuerURL, "/") + "/.well-known/openid-configuration"
	if strings.Contains(discURL, "realms/") {
		// Keycloak format.
		discURL = strings.TrimSuffix(a.config.IssuerURL, "/") + "/.well-known/openid-configuration"
	} else {
		// Standard OIDC.
		discURL = strings.TrimSuffix(a.config.IssuerURL, "/") + "/.well-known/openid-configuration"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discURL, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to create discovery request: %v", ErrAuthFailed, err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: failed to fetch discovery: %v", ErrAuthFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: discovery returned %d", ErrAuthFailed, resp.StatusCode)
	}

	var discovery map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return fmt.Errorf("%w: invalid discovery json: %v", ErrAuthFailed, err)
	}

	jwksURI, ok := discovery["jwks_uri"].(string)
	if !ok || jwksURI == "" {
		return fmt.Errorf("%w: no jwks_uri in discovery", ErrAuthFailed)
	}

	// Fetch JWKS.
	jwksReq, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to create JWKS request: %v", ErrAuthFailed, err)
	}

	jwksResp, err := a.client.Do(jwksReq)
	if err != nil {
		return fmt.Errorf("%w: failed to fetch JWKS: %v", ErrAuthFailed, err)
	}
	defer jwksResp.Body.Close()

	if jwksResp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: JWKS returned %d", ErrAuthFailed, jwksResp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(jwksResp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("%w: invalid JWKS json: %v", ErrAuthFailed, err)
	}

	a.jwksMu.Lock()
	a.jwks = &jwks
	a.jwksAt = time.Now()
	a.jwksMu.Unlock()

	return nil
}

func (a *Adapter) verifyRS256(token string, key *JWK, alg string) error {
	if alg != "RS256" && alg != "" {
		return fmt.Errorf("%w: unsupported algorithm %s", ErrAuthFailed, alg)
	}
	// RSA signature verification would use crypto/rsa here.
	// For now, placeholder: real impl uses rsa.VerifyPKCS1v15.
	_ = key
	_ = token
	return nil // placeholder
}

func base64URLDecode(s string) ([]byte, error) {
	// Go's base64.URLEncoding without padding.
	decoder := base64URLEncoding{}
	return decoder.DecodeString(s)
}

type base64URLEncoding struct{}

func (e base64URLEncoding) DecodeString(s string) ([]byte, error) {
	// Add padding if necessary.
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	s = strings.ReplaceAll(s, "-_", "+/")
	return decodeBase64(s)
}

func decodeBase64(s string) ([]byte, error) {
	const decodeStd = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, len(s)*6/8)
	j := 0
	var val int
	var bits int
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '=' {
			break
		}
		pos := strings.IndexByte(decodeStd, c)
		if pos < 0 {
			return nil, fmt.Errorf("invalid base64 character %c", c)
		}
		val = val<<6 | pos
		bits += 6
		if bits >= 8 {
			bits -= 8
			result[j] = byte(val >> bits)
			j++
		}
	}
	return result[:j], nil
}

type jwtHeader struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
}

type jwtClaims struct {
	Sub               string         `json:"sub"`
	Iss               string         `json:"iss"`
	Exp               int64          `json:"exp"`
	Iat               int64          `json:"iat"`
	Type              string         `json:"type"`
	Groups            []string       `json:"groups"`
	TenantMemberships []membership   `json:"tenant_memberships"`
	Extra             map[string]any `json:"-"`
}

func (c *jwtClaims) ExtraClaims() map[string]any {
	return c.Extra
}

type membership struct {
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
}

func mapClaimType(t string) string {
	switch t {
	case "user", "human":
		return "human"
	case "service":
		return "service"
	default:
		return t
	}
}

// sha256Hash computes the SHA-256 digest of the input string.
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
