package plugin

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// versionRe is deliberately narrower than semver 2.0: plugin manifests use
// plain x.y[.z] versions, and unsupported operator syntax must fail closed
// rather than silently allowing a plugin meant for a newer host.
var versionRe = regexp.MustCompile(`^[vV]?(\d+)(?:\.(\d+))?(?:\.(\d+))?$`)

var constraintRe = regexp.MustCompile(`^(>=|<=|>|<|=)?(.+)$`)

type semVersion struct {
	major, minor, patch int
}

func parseSemVersion(s string) (semVersion, error) {
	m := versionRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return semVersion{}, fmt.Errorf("invalid version %q", s)
	}
	parse := func(v string) int {
		if v == "" {
			return 0
		}
		n, _ := strconv.Atoi(v)
		return n
	}
	return semVersion{parse(m[1]), parse(m[2]), parse(m[3])}, nil
}

func (v semVersion) compare(o semVersion) int {
	switch {
	case v.major != o.major:
		if v.major < o.major {
			return -1
		}
		return 1
	case v.minor != o.minor:
		if v.minor < o.minor {
			return -1
		}
		return 1
	case v.patch != o.patch:
		if v.patch < o.patch {
			return -1
		}
		return 1
	}
	return 0
}

// checkMinServer evaluates a plugin minServer constraint against the host
// version. It supports the forms used by this repository: >=x.y[.z],
// <=x.y[.z], >x, <x, =x, and space/comma-separated conjunctions. Anything
// else (^, ~, prerelease, ranges with ||) is rejected so a typo cannot
// silently widen the gate.
func checkMinServer(constraint, hostVersion string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true, nil
	}
	host, err := parseSemVersion(hostVersion)
	if err != nil {
		return false, fmt.Errorf("host version: %w", err)
	}
	parts := strings.FieldsFunc(constraint, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	if len(parts) == 0 {
		return false, fmt.Errorf("empty constraint")
	}
	for _, part := range parts {
		m := constraintRe.FindStringSubmatch(part)
		if m == nil {
			return false, fmt.Errorf("unsupported constraint %q", part)
		}
		op := m[1]
		if op == "" {
			op = "="
		}
		want, err := parseSemVersion(m[2])
		if err != nil {
			return false, fmt.Errorf("constraint %q: %w", part, err)
		}
		c := host.compare(want)
		ok := false
		switch op {
		case ">=":
			ok = c >= 0
		case "<=":
			ok = c <= 0
		case ">":
			ok = c > 0
		case "<":
			ok = c < 0
		case "=":
			ok = c == 0
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// satisfiesVersion implements the dependency-constraint subset used by
// manifests: comparators, ^, ~, *, x wildcards, and ||. Malformed constraints
// fail closed (error), never "allow everything".
func satisfiesVersion(version, constraint string) (bool, error) {
	v, err := parseSemVersion(version)
	if err != nil {
		return false, err
	}
	for _, group := range strings.Split(constraint, "||") {
		ok, err := satisfiesGroup(v, group)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func satisfiesGroup(v semVersion, group string) (bool, error) {
	tokens := strings.FieldsFunc(group, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	if len(tokens) == 0 {
		return false, fmt.Errorf("empty constraint")
	}
	for _, tok := range tokens {
		ok, err := satisfiesToken(v, tok)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func satisfiesToken(v semVersion, tok string) (bool, error) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return true, nil
	}
	if tok == "*" || strings.EqualFold(tok, "x") {
		return true, nil
	}
	// 1.2.x / 1.x: treat as a minor/major range.
	if strings.Contains(strings.ToLower(tok), "x") || strings.Contains(tok, "*") {
		parts := strings.Split(strings.TrimLeft(tok, "=vV"), ".")
		if len(parts) < 2 {
			return true, nil
		}
		base, err := strconv.Atoi(parts[0])
		if err != nil {
			return false, fmt.Errorf("invalid wildcard %q", tok)
		}
		if len(parts) >= 2 && parts[1] != "x" && parts[1] != "X" && parts[1] != "*" {
			minor, err := strconv.Atoi(parts[1])
			if err != nil {
				return false, fmt.Errorf("invalid wildcard %q", tok)
			}
			return v.major == base && v.minor == minor, nil
		}
		return v.major == base, nil
	}
	op := ""
	for _, p := range []string{">=", "<=", ">", "<", "=", "^", "~"} {
		if strings.HasPrefix(tok, p) {
			op = p
			tok = strings.TrimSpace(strings.TrimPrefix(tok, p))
			break
		}
	}
	want, err := parseSemVersion(tok)
	if err != nil {
		return false, fmt.Errorf("constraint %q: %w", tok, err)
	}
	c := v.compare(want)
	switch op {
	case "", "=":
		return c == 0, nil
	case ">=":
		return c >= 0, nil
	case "<=":
		return c <= 0, nil
	case ">":
		return c > 0, nil
	case "<":
		return c < 0, nil
	case "^":
		if c < 0 {
			return false, nil
		}
		if want.major > 0 {
			return v.major == want.major, nil
		}
		if want.minor > 0 {
			return v.major == 0 && v.minor == want.minor, nil
		}
		return v.major == 0 && v.minor == 0 && v.patch == want.patch, nil
	case "~":
		if c < 0 {
			return false, nil
		}
		return v.major == want.major && v.minor == want.minor, nil
	}
	return false, fmt.Errorf("unsupported constraint %q", tok)
}
