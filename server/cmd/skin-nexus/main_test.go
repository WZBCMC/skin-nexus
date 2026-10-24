package main

import "testing"

// 拍板（dev-plan §9）: option site_url → legacy option base_url → env。
func TestResolveBaseURL(t *testing.T) {
	cases := []struct {
		name string
		opts map[string]string
		env  string
		want string
	}{
		{"site_url wins over base_url and env",
			map[string]string{"site_url": "https://new.example", "base_url": "https://old.example"},
			"http://env.example", "https://new.example"},
		{"legacy base_url used when site_url absent",
			map[string]string{"base_url": "https://old.example"},
			"http://env.example", "https://old.example"},
		{"env fallback when both options absent", map[string]string{},
			"http://env.example", "http://env.example"},
		{"non-http site_url rejected -> env",
			map[string]string{"site_url": "ftp://evil"},
			"http://env.example", "http://env.example"},
		{"garbage base_url rejected -> env",
			map[string]string{"base_url": "not a url"},
			"http://env.example", "http://env.example"},
		{"trailing slashes trimmed",
			map[string]string{"site_url": "https://s.example/"},
			"", "https://s.example"},
		{"empty site_url falls through to base_url",
			map[string]string{"site_url": "", "base_url": "https://old.example"},
			"http://env.example", "https://old.example"},
	}
	for _, c := range cases {
		got := resolveBaseURL(func(key string) (string, bool) {
			v, ok := c.opts[key]
			return v, ok
		}, c.env)
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
