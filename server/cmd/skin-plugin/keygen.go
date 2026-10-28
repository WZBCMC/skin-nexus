package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func runKeygen(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "key id used in SKIN_PLUGIN_TRUST_KEYS")
	out := fs.String("out", "", "private key PEM path (keep offline)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" || *out == "" {
		fmt.Fprintln(stderr, "keygen: --id and --out are required")
		return 2
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o700); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		f.Close()
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stderr, "private key written to %s (mode 0600); keep it offline and back it up\n", *out)
	fmt.Fprintf(stdout, "%s:%s\n", *id, base64.StdEncoding.EncodeToString(pub))
	return 0
}
