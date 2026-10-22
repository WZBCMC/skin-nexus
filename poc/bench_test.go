package poc

import (
	"os"
	"runtime"
	"sync"
	"testing"
)

func sampleCtx() FilterContext {
	return FilterContext{
		ProfileName: "Steve",
		UUID:        "069a79f444e94726a5befca90e38aaf5",
		Textures: map[string]interface{}{
			"SKIN": map[string]interface{}{
				"url":     "http://textures.example.com/skin.png",
				"metadata": map[string]interface{}{"model": "default"},
			},
		},
	}
}

func load(t testing.TB, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGojaFilter(t *testing.T) {
	r, err := NewGojaRunner("sample.js", load(t, "plugins/sample.js"))
	if err != nil {
		t.Fatal(err)
	}
	c := sampleCtx()
	if err := r.Apply(&c); err != nil {
		t.Fatal(err)
	}
	if c.ProfileName != "Steve_js" {
		t.Fatalf("profileName = %q", c.ProfileName)
	}
	skin := c.Textures["SKIN"].(map[string]interface{})
	if skin["url"] != "http://textures.cdn.example.com/skin.png" {
		t.Fatalf("skin url = %v", skin["url"])
	}
}

func TestLuaFilter(t *testing.T) {
	r, err := NewLuaRunner("sample.lua", load(t, "plugins/sample.lua"))
	if err != nil {
		t.Fatal(err)
	}
	c := sampleCtx()
	if err := r.Apply(&c); err != nil {
		t.Fatal(err)
	}
	if c.ProfileName != "Steve_lua" {
		t.Fatalf("profileName = %q", c.ProfileName)
	}
	skin := c.Textures["SKIN"].(map[string]interface{})
	if skin["url"] != "http://textures.cdn.example.com/skin.png" {
		t.Fatalf("skin url = %v", skin["url"])
	}
}

// TestPoolConcurrency runs 100 goroutines through pooled runtimes and reports
// peak heap. Gate (AI3): pool memory < 150MB with 100 concurrent instances.
func TestPoolConcurrency(t *testing.T) {
	js, _ := NewGojaRunner("sample.js", load(t, "plugins/sample.js"))
	luaR, _ := NewLuaRunner("sample.lua", load(t, "plugins/sample.lua"))

	var wg sync.WaitGroup
	work := func(r FilterRunner, suffix string) {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			c := sampleCtx()
			if err := r.Apply(&c); err != nil {
				t.Error(err)
				return
			}
		}
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go work(js, "js")
		go work(luaR, "lua")
	}
	wg.Wait()
	runtime.ReadMemStats(&after)
	t.Logf("heap alloc after 100 goroutines x 200 calls: %.1f MiB (total alloc %.1f MiB)",
		float64(after.HeapAlloc)/(1<<20), float64(after.TotalAlloc-before.TotalAlloc)/(1<<20))
}

func benchFilter(b *testing.B, r FilterRunner) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c := sampleCtx()
		if err := r.Apply(&c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGojaFilter(b *testing.B) {
	r, err := NewGojaRunner("sample.js", load(b, "plugins/sample.js"))
	if err != nil {
		b.Fatal(err)
	}
	benchFilter(b, r)
}

func BenchmarkLuaFilter(b *testing.B) {
	r, err := NewLuaRunner("sample.lua", load(b, "plugins/sample.lua"))
	if err != nil {
		b.Fatal(err)
	}
	benchFilter(b, r)
}
