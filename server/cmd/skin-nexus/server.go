package main

import (
	"net/http"
	"time"
)

// newHTTPServer builds the public listener with connection deadlines.
// http.ListenAndServe's zero-value server has no timeouts: a slow client can
// hold a header/body read open indefinitely and exhaust fds/goroutines
// (2026-10-25 load-audit finding). Read/Write deadlines are generous because
// texture uploads run over shared campus NAT links; header/idle deadlines
// carry the slowloris protection.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
