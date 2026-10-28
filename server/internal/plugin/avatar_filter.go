package plugin

import (
	"context"
	"net"
	"net/url"
	"strings"
)

const maxAvatarURLLen = 2048

type avatarTarget struct {
	fn    avatarHookFn
	hosts []string
}

// AvatarURL asks enabled plugins, in name order, whether an avatar should be
// served from an external host. A URL is accepted only if it is HTTPS, has no
// userinfo/non-default port, and its host is declared by the same plugin's
// manifest. Anything else falls through to the host-rendered PNG.
func (m *Manager) AvatarURL(ctx context.Context, hash string, size int, threeD bool) (string, bool) {
	if hash == "" {
		return "", false
	}
	m.mu.RLock()
	targets := make([]avatarTarget, 0, len(m.extra.avatar))
	for _, h := range m.extra.avatar {
		targets = append(targets, avatarTarget{fn: h.fn})
	}
	for _, name := range m.order {
		inst := m.instances[name]
		if !inst.enabled || inst.rt == nil || inst.info.Media == nil {
			continue
		}
		hosts := append([]string(nil), inst.info.Media.AvatarHosts...)
		for _, h := range inst.rt.hooks.avatar {
			targets = append(targets, avatarTarget{fn: h.fn, hosts: hosts})
		}
	}
	m.mu.RUnlock()

	for _, t := range targets {
		f := AvatarFilter{Hash: hash, Size: size, ThreeD: threeD}
		if err := t.fn(ctx, &f); err != nil || f.URL == "" {
			continue
		}
		if !validAvatarURL(f.URL, t.hosts) {
			continue
		}
		return f.URL, true
	}
	return "", false
}

func validAvatarURL(raw string, hosts []string) bool {
	if raw == "" || len(raw) > maxAvatarURLLen || len(hosts) == 0 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" {
		return false
	}
	if p := u.Port(); p != "" && p != "443" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || net.ParseIP(host) != nil {
		return false
	}
	for _, allowed := range hosts {
		allowed = strings.ToLower(allowed)
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
			continue
		}
		if host == allowed {
			return true
		}
	}
	return false
}
