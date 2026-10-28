package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"skin-nexus/server/internal/signedpkg"
)

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%s is not a PEM file", path)
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s is not an Ed25519 private key", path)
	}
	return priv, nil
}

func publicKeyString(priv ed25519.PrivateKey) string {
	pub := priv.Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(pub)
}

func keyringFor(keys, keyID string, priv ed25519.PrivateKey) (signedpkg.Keyring, error) {
	ring := signedpkg.Keyring{}
	if keys != "" {
		parsed, err := signedpkg.ParseKeys(keys)
		if err != nil {
			return nil, err
		}
		for k, v := range parsed {
			ring[k] = v
		}
	}
	if priv != nil {
		if keyID == "" {
			return nil, fmt.Errorf("--key-id is required with --key")
		}
		ring[keyID] = priv.Public().(ed25519.PublicKey)
	}
	if len(ring) == 0 {
		return nil, fmt.Errorf("no trust keys: pass --keys or set SKIN_PLUGIN_TRUST_KEYS")
	}
	return ring, nil
}
