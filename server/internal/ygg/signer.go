// Package ygg implements the Yggdrasil API surface and texture signing.
// Signer abstracts the signing key source: phase 1 ships LocalSigner only;
// UnionSigner (MUA hub-distributed shared key) is a phase-2 stub behind the
// same interface (blueprint §2.5 / §5).
package ygg

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// Signer produces textures-property signatures and exposes the verification
// public key for the metadata endpoint.
type Signer interface {
	// SignValue signs the Base64 textures value string (SHA1withRSA) and
	// returns the Base64 signature, matching yggdrasil-connect semantics.
	SignValue(value string) (string, error)
	// PublicKeyPEM returns the X.509 SubjectPublicKeyInfo PEM served as
	// `signaturePublickey`. Format is strict: authlib-injector exits the
	// game on malformed PEM.
	PublicKeyPEM() string
	// KeyFingerprint identifies the active key in admin UI / audit logs.
	KeyFingerprint() string
}

// LocalSigner holds a locally generated RSA key pair (phase 1).
type LocalSigner struct {
	key       *rsa.PrivateKey
	publicPEM string
	fp        string
}

// NewLocalSigner loads a PKCS#8 PEM private key from path, or generates and
// persists one when the file does not exist (0600).
func NewLocalSigner(path string) (*LocalSigner, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		key, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, err
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, err
		}
		pemBytes = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
			return nil, err
		}
		return newLocalSigner(key)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("ygg: %s: no PEM block", path)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("ygg: %s: %w", path, err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("ygg: %s: not an RSA key", path)
	}
	return newLocalSigner(key)
}

func newLocalSigner(key *rsa.PrivateKey) (*LocalSigner, error) {
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return &LocalSigner{
		key:       key,
		publicPEM: string(pubPEM),
		fp:        fingerprint(der),
	}, nil
}

func (s *LocalSigner) SignValue(value string) (string, error) {
	digest := sha1.Sum([]byte(value))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA1, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func (s *LocalSigner) PublicKeyPEM() string { return s.publicPEM }

func (s *LocalSigner) KeyFingerprint() string { return s.fp }

// fingerprint is the SHA-256 prefix of the DER public key, shown in admin UI.
func fingerprint(der []byte) string {
	h := sha256.Sum256(der)
	return fmt.Sprintf("SHA256:%x", h[:8])
}
