package plugin

import "testing"

func TestSatisfiesVersion(t *testing.T) {
	cases := []struct {
		v, c string
		want bool
	}{
		{"1.2.3", ">=1.0.0 <2.0.0", true},
		{"2.0.0", ">=1.0.0 <2.0.0", false},
		{"1.5.0", "^1.2.0", true},
		{"2.0.0", "^1.2.0", false},
		{"1.2.9", "~1.2.3", true},
		{"1.3.0", "~1.2.3", false},
		{"1.2.3", "1.2.x", true},
		{"1.3.0", "1.2.x", false},
		{"1.2.3", "*", true},
		{"1.2.3", ">=1.2.0 || <1.0.0", true},
	}
	for _, tc := range cases {
		got, err := satisfiesVersion(tc.v, tc.c)
		if err != nil || got != tc.want {
			t.Fatalf("satisfies(%s,%s)=%v,%v want %v", tc.v, tc.c, got, err, tc.want)
		}
	}
	if _, err := satisfiesVersion("1.0.0", "not-a-version"); err == nil {
		t.Fatal("malformed constraint accepted")
	}
}

func TestPlanInstallMissingAndConflict(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "dep", `{"name":"dep","version":"1.0.0","hookApi":2,"entrypoint":"index.js","permissions":[]}`, "skin.log('x');")
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	p, err := m.PlanInstall(PlanInput{Name: "new", Version: "1.0.0", Requires: &RequiresSpec{
		Plugins: map[string]string{"dep": ">=2.0.0"},
	}})
	if err != nil || p.CanInstall || len(p.Missing) == 0 {
		t.Fatalf("missing plan = %+v err=%v", p, err)
	}
	p, err = m.PlanInstall(PlanInput{Name: "new2", Version: "1.0.0", Conflicts: []string{"dep"}})
	if err != nil || p.CanInstall || len(p.Conflicts) == 0 {
		t.Fatalf("conflict plan = %+v err=%v", p, err)
	}
	p, err = m.PlanInstall(PlanInput{Name: "new3", Version: "1.0.0", Requires: &RequiresSpec{
		Plugins: map[string]string{"dep": "^1.0.0"},
	}})
	if err != nil || !p.CanInstall {
		t.Fatalf("satisfied plan = %+v err=%v", p, err)
	}
}
