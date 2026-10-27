package main

import (
	"net/http"
	"testing"
)

func TestNewHTTPServerHardening(t *testing.T) {
	s := newHTTPServer(":0", http.NotFoundHandler())
	if s.ReadHeaderTimeout <= 0 || s.ReadTimeout <= 0 || s.WriteTimeout <= 0 || s.IdleTimeout <= 0 {
		t.Fatalf("timeouts must be positive: header=%v read=%v write=%v idle=%v",
			s.ReadHeaderTimeout, s.ReadTimeout, s.WriteTimeout, s.IdleTimeout)
	}
	if s.MaxHeaderBytes <= 0 {
		t.Fatalf("MaxHeaderBytes = %d, want positive", s.MaxHeaderBytes)
	}
}
