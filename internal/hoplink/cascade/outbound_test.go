package cascade

import (
	"sync"
	"testing"
)

func TestParseCurrentGoroutineID(t *testing.T) {
	if id := parseCurrentGoroutineID(); id == 0 {
		t.Fatal("expected non-zero goroutine id")
	}
}

func TestEntryRouterRoutePerGoroutine(t *testing.T) {
	domain, err := LoadPatternsFromText("domain:ru")
	if err != nil {
		t.Fatal(err)
	}
	sni, err := LoadPatternsFromText("full:video.google.com")
	if err != nil {
		t.Fatal(err)
	}
	r := newEntryRouter("next:443", domain, sni, nil)

	var wg sync.WaitGroup
	const n = 64
	results := make([]bool, n)
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			local := idx%2 == 0
			r.storeRoute(local)
			results[idx] = r.pickLocal("blizzard.com:443")
		}(i)
	}
	wg.Wait()
	for i, got := range results {
		want := i%2 == 0
		if got != want {
			t.Fatalf("goroutine %d: pickLocal=%v want %v", i, got, want)
		}
	}
}

func TestEntryRouterManyConcurrentClients(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	domain, err := LoadPatternsFromText("domain:ru")
	if err != nil {
		t.Fatal(err)
	}
	r := newEntryRouter("next:443", domain, Matcher{}, nil)

	var wg sync.WaitGroup
	const workers = 4096
	errCh := make(chan struct{}, 1)
	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			local := idx%3 == 0
			r.storeRoute(local)
			got := r.pickLocal("example.net:443")
			if got != local {
				select {
				case errCh <- struct{}{}:
				default:
				}
			}
		}(i)
	}
	wg.Wait()
	select {
	case <-errCh:
		t.Fatal("concurrent route mismatch")
	default:
	}
}