package plugin

import "testing"

func TestCheckMinServer(t *testing.T) {
	cases := []struct {
		name       string
		constraint string
		host       string
		want       bool
		wantErr    bool
	}{
		{"exact", "0.4.0", "0.4.0", true, false},
		{"exact miss", "0.4.0", "0.4.1", false, false},
		{"range match", ">=0.4.0 <1.0.0", "0.4.0", true, false},
		{"range upper", ">=0.4.0 <1.0.0", "0.9.9", true, false},
		{"range too low", ">=0.4.0 <1.0.0", "0.3.9", false, false},
		{"range too high", ">=0.4.0 <1.0.0", "1.0.0", false, false},
		{"short minor", ">=0.4", "0.4.7", true, false},
		{"v prefix", ">=v0.4.0", "v0.4.2", true, false},
		{"comma", ">=0.4.0, <0.6.0", "0.5.0", true, false},
		{"caret unsupported", "^0.4.0", "0.4.2", false, true},
		{"tilde unsupported", "~0.4.0", "0.4.2", false, true},
		{"garbage", "latest", "0.4.0", false, true},
		{"bad host", ">=0.4.0", "main", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := checkMinServer(tc.constraint, tc.host)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("check(%q,%q)=%v want %v", tc.constraint, tc.host, got, tc.want)
			}
		})
	}
}
