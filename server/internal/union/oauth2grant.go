package union

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// Union OAuth2 grant token (analysis/yggdrasil-connect.md §4.5, byte-order
// frozen against UnionOAuth2Controller.php:38-79):
//
//  1. userInfo  = base64(json{uid,nickname,email,expires_at: now+600})
//  2. mac       = hex(HMAC-SHA256(key=union_member_key, msg=userInfo))   (the
//     message is the base64 string itself, not the JSON)
//  3. signature = base64(RSA-SHA256 PKCS#1 v1.5 over "userInfo + '.' + mac",
//     signed with union_oauth2_sig_private_key)
//  4. token     = json{userInfo,mac,signature} encrypted with the hub RSA
//     public key in PKCS#1 v1.5 chunks (chunk = keyBytes-11),
//     concatenated and base64'd as a whole
//  5. redirect  = {root}/oauth2/continue?userInfoToken=<token>&<original query>
const grantTokenTTLSeconds = 600 // PHP $userInfoTokenTTL

// Grantor builds Union OAuth2 grant tokens and serves this site's grant
// signature public key. HMAC key rotation uses an atomic value (read on every
// grant, written only by updatebackendkey).
type Grantor struct {
	sigKey    *rsa.PrivateKey
	sigPubPEM string
	key       atomic.Value // string: union member key (HMAC key)
	origin    string       // hub origin for CORS
	enable    bool         // module + unionOAuth2 both enabled
	oauth2    bool         // unionOAuth2 feature enabled (403 vs 404 semantics)
	now       func() time.Time
}

// NewGrantor loads the member's OAuth2 signing pair (PKCS#8/PKCS#1 private,
// PKIX public). When UnionOAuth2 is off the returned Grantor is inert:
// GrantToken fails with ErrDisabled and the key endpoint answers 403/404.
func NewGrantor(cfg Config) (*Grantor, error) {
	g := &Grantor{
		origin: cfg.hubOrigin(),
		enable: cfg.Enabled && cfg.UnionOAuth2,
		oauth2: cfg.UnionOAuth2,
		now:    time.Now,
	}
	g.key.Store(cfg.MemberKey)
	if !cfg.Enabled {
		return g, nil
	}
	if !cfg.UnionOAuth2 {
		return g, nil // inert: feature switched off
	}
	key, err := parsePrivateKeyPEM(cfg.SigPrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("union: OAuth2 signature key: %w", err)
	}
	// Keep the configured PEM verbatim: PKIX PEM formatting is strict for
	// consumers (authlib-injector exits on malformed PEM), so no trimming.
	var pubPEM string
	if strings.TrimSpace(cfg.SigPublicKeyPEM) == "" {
		derived, _, err := publicPEM(&key.PublicKey)
		if err != nil {
			return nil, err
		}
		pubPEM = derived
	} else if _, err := parsePublicKeyPEM(cfg.SigPublicKeyPEM); err != nil {
		return nil, fmt.Errorf("union: OAuth2 signature public key: %w", err)
	} else {
		pubPEM = cfg.SigPublicKeyPEM
	}
	g.sigKey = key
	g.sigPubPEM = pubPEM
	return g, nil
}

// Enabled reports whether grant issuance is live (nil-safe).
func (g *Grantor) Enabled() bool { return g != nil && g.enable }

// SetMemberKey swaps the HMAC key after a member-key rotation.
func (g *Grantor) SetMemberKey(key string) {
	if g != nil {
		g.key.Store(key)
	}
}

// SignaturePublicKeyPEM returns this site's grant-signature public key
// (served at GET /api/union/member/oauth2/ as {"signaturePublicKey":...}).
func (g *Grantor) SignaturePublicKeyPEM() string {
	if g == nil {
		return ""
	}
	return g.sigPubPEM
}

// userInfo is the signed user claim set. Field order is frozen: it is
// serialized into the base64 payload the hub parses.
type userInfo struct {
	UID       int64  `json:"uid"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expires_at"` // Unix seconds, now+600
}

// GrantTokenDoc is the plaintext token document before hub-key encryption
// (json{userInfo,mac,signature}, field order frozen).
type GrantTokenDoc struct {
	UserInfo  string `json:"userInfo"`
	MAC       string `json:"mac"`
	Signature string `json:"signature"`
}

// GrantToken builds the encrypted userInfoToken for one logged-in user.
// hubPublicKeyPEM is the hub RSA encryption key, normally fetched with
// Outbound.OAuth2BackendKey (GET {root}/oauth2/backend -> {publicKey}).
func (g *Grantor) GrantToken(uid int64, nickname, email, hubPublicKeyPEM string) (string, error) {
	if !g.Enabled() {
		return "", ErrDisabled
	}
	hubPub, err := parsePublicKeyPEM(hubPublicKeyPEM)
	if err != nil {
		return "", fmt.Errorf("union: hub encryption key: %w", err)
	}

	now := g.now()
	info := userInfo{
		UID:       uid,
		Nickname:  nickname,
		Email:     email,
		ExpiresAt: now.Unix() + grantTokenTTLSeconds,
	}
	infoJSON, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("union: encode user info: %w", err)
	}
	infoB64 := base64.StdEncoding.EncodeToString(infoJSON)

	mac := grantMAC(infoB64, g.memberKey())

	digest := sha256.Sum256([]byte(infoB64 + "." + mac))
	sig, err := rsa.SignPKCS1v15(rand.Reader, g.sigKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("union: sign grant token: %w", err)
	}

	doc := GrantTokenDoc{
		UserInfo:  infoB64,
		MAC:       mac,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	tokenJSON, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("union: encode grant token: %w", err)
	}
	enc, err := EncryptToken(hubPub, tokenJSON)
	if err != nil {
		return "", fmt.Errorf("union: encrypt grant token: %w", err)
	}
	return enc, nil
}

func (g *Grantor) memberKey() string {
	s, _ := g.key.Load().(string)
	return s
}

// grantMAC is hex(HMAC-SHA256(member key, userInfoBase64)).
func grantMAC(userInfoB64, memberKey string) string {
	m := hmac.New(sha256.New, []byte(memberKey))
	m.Write([]byte(userInfoB64))
	return hex.EncodeToString(m.Sum(nil))
}

// GrantRedirectURL renders the hub continue-URL:
// {continueBase}?userInfoToken=<token>&<original query>. userInfoToken is
// always first and always ours (a caller-supplied one is dropped).
// NOTE (deviation, reported): the spec signature omitted the token parameter;
// a stateless builder must receive the token from the caller, so it is the
// first argument here.
func (g *Grantor) GrantRedirectURL(token, continueBase string, rawQuery url.Values) string {
	sep := "?"
	if strings.Contains(continueBase, "?") {
		sep = "&"
	}
	var b strings.Builder
	b.WriteString(continueBase)
	b.WriteString(sep)
	b.WriteString("userInfoToken=")
	b.WriteString(url.QueryEscape(token))

	keys := make([]string, 0, len(rawQuery))
	for k := range rawQuery {
		if k == "userInfoToken" {
			continue // never let the original query override the token
		}
		keys = append(keys, k)
	}
	sort.Strings(keys) // url.Values has no order; sorted keeps output stable
	for _, k := range keys {
		for _, v := range rawQuery[k] {
			b.WriteByte('&')
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(v))
		}
	}
	return b.String()
}

// SigKeyHandler serves GET /api/union/member/oauth2/ — the grant signature
// public document for the hub, CORS-restricted to the hub origin
// (UnionOAuth2Controller.php:26-33,81-87).
func (g *Grantor) SigKeyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g == nil || !g.oauth2 {
			// Feature off (legacy middleware parity).
			writeError(w, http.StatusForbidden, codeForbidden, "Union OAuth2 is not enabled on this server.")
			return
		}
		if !g.enable {
			// Whole module off: zero surface.
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		if g.origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", g.origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		}
		writeJSON(w, http.StatusOK, map[string]string{"signaturePublicKey": g.SignaturePublicKeyPEM()})
	})
}

// ---- chunked RSA public-key encryption (RSAPublicTrait.php:22-35) ----

// EncryptToken encrypts plaintext with a raw RSA public key in PKCS#1 v1.5
// chunks of (keyBytes - 11) bytes, concatenates the ciphertext chunks and
// base64-encodes the whole blob. Exported for hub-side round-trip tests.
func EncryptToken(pub *rsa.PublicKey, plaintext []byte) (string, error) {
	if pub == nil {
		return "", errors.New("union: nil encryption key")
	}
	keyBytes := pub.Size()
	chunkSize := keyBytes - 11 // PKCS#1 v1.5 padding budget
	if chunkSize <= 0 {
		return "", errors.New("union: RSA key too small")
	}
	var out []byte
	for len(plaintext) > 0 {
		n := chunkSize
		if n > len(plaintext) {
			n = len(plaintext)
		}
		chunk, err := rsa.EncryptPKCS1v15(rand.Reader, pub, plaintext[:n])
		if err != nil {
			return "", fmt.Errorf("union: encrypt chunk: %w", err)
		}
		out = append(out, chunk...)
		plaintext = plaintext[n:]
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptToken is the inverse of EncryptToken: base64-decode, split into
// keyBytes ciphertext chunks, PKCS#1 v1.5 decrypt each, concatenate.
func DecryptToken(priv *rsa.PrivateKey, encoded string) ([]byte, error) {
	if priv == nil {
		return nil, errors.New("union: nil decryption key")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("union: token is not base64: %w", err)
	}
	keyBytes := priv.Size()
	if keyBytes <= 0 {
		return nil, errors.New("union: invalid RSA key")
	}
	var out []byte
	for len(raw) > 0 {
		n := keyBytes
		if n > len(raw) {
			n = len(raw)
		}
		plain, err := rsa.DecryptPKCS1v15(rand.Reader, priv, raw[:n])
		if err != nil {
			return nil, fmt.Errorf("union: decrypt chunk: %w", err)
		}
		out = append(out, plain...)
		raw = raw[n:]
	}
	return out, nil
}
