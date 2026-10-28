package outbound

import (
	"net"
	"testing"
)

func TestHostAllowed(t *testing.T) {
	allow := []string{"api.example.com", "*.cdn.example.com"}
	for _, h := range []string{"api.example.com", "a.cdn.example.com", "A.CDN.EXAMPLE.COM"} {
		if !hostAllowed(h, allow) {
			t.Fatalf("%s should be allowed", h)
		}
	}
	for _, h := range []string{"example.com", "cdn.example.com", "evil-api.example.com", "api.example.com.evil"} {
		if hostAllowed(h, allow) {
			t.Fatalf("%s should be denied", h)
		}
	}
}

func TestPublicIP(t *testing.T) {
	public := []string{"8.8.8.8", "2001:4860:4860::8888"}
	private := []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.1.1", "100.64.0.1", "::1", "fc00::1", "fe80::1", "0.0.0.0"}
	for _, s := range public {
		if !isPublicIP(net.ParseIP(s)) {
			t.Fatalf("%s should be public", s)
		}
	}
	for _, s := range private {
		if isPublicIP(net.ParseIP(s)) {
			t.Fatalf("%s should be private", s)
		}
	}
}

func TestGetRejectsDisallowedHost(t *testing.T) {
	c := New([]string{"allowed.example"}, 1000000000, 10)
	if _, err := c.Get(nil, "http://127.0.0.1:8080/x"); err == nil {
		t.Fatal("disallowed host accepted")
	}
}
