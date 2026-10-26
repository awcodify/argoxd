package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewerComparesReleaseNumbers(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"0.4.1", "v0.4.2", true},
		{"v0.4.1", "0.4.1", false},
		{"0.4.2", "0.4.1", false},
		{"0.9.0", "0.10.0", true},
		{"0.4.1", "1.0.0", true},
		{"dev", "0.4.1", false},
		{"v0.3.1-0.20261022034504-e3a2b675f4d1+dirty", "0.4.1", false},
		{"0.4.1", "not-a-version", false},
		{"0.4.1", "", false},
	}
	for _, test := range tests {
		if got := Newer(test.current, test.latest); got != test.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", test.current, test.latest, got, test.want)
		}
	}
}

func releaseServer(t *testing.T, tag string, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"tag_name":"` + tag + `"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func newChecker(t *testing.T, url string, now *time.Time) Checker {
	t.Helper()
	return Checker{
		URL:       url,
		CachePath: filepath.Join(t.TempDir(), "argoxd", "update.json"),
		Interval:  24 * time.Hour,
		Now:       func() time.Time { return *now },
	}
}

func TestLatestFetchesAndCachesForTheInterval(t *testing.T) {
	var hits atomic.Int32
	server := releaseServer(t, "v0.4.2", &hits)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	checker := newChecker(t, server.URL, &now)

	for range 2 {
		latest, err := checker.Latest(context.Background())
		if err != nil || latest != "0.4.2" {
			t.Fatalf("Latest() = %q, %v; want 0.4.2", latest, err)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("GitHub was asked %d times within the interval, want 1", hits.Load())
	}

	now = now.Add(25 * time.Hour)
	if _, err := checker.Latest(context.Background()); err != nil || hits.Load() != 2 {
		t.Fatalf("after the interval: hits = %d, err = %v; want a second request", hits.Load(), err)
	}
}

func TestLatestFailsQuietlyAndKeepsTheCache(t *testing.T) {
	var hits atomic.Int32
	server := releaseServer(t, "v0.4.2", &hits)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	checker := newChecker(t, server.URL, &now)
	if _, err := checker.Latest(context.Background()); err != nil {
		t.Fatal(err)
	}

	server.Close()
	now = now.Add(48 * time.Hour)
	if latest, err := checker.Latest(context.Background()); err == nil || latest != "" {
		t.Fatalf("Latest() = %q, %v; want an error and no version when GitHub is unreachable", latest, err)
	}
	if _, err := os.Stat(checker.CachePath); err != nil {
		t.Fatalf("a failed check removed the cache: %v", err)
	}
}

func TestLatestRejectsBadResponses(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"rate limited": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "limit", http.StatusForbidden) },
		"not json":     func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) },
		"no tag":       func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) },
	} {
		server := httptest.NewServer(handler)
		now := time.Now()
		if latest, err := newChecker(t, server.URL, &now).Latest(context.Background()); err == nil {
			t.Errorf("%s: Latest() = %q, nil; want an error", name, latest)
		}
		server.Close()
	}
}

func TestLatestGivesUpWhenTheContextExpires(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	started := time.Now()
	if _, err := newChecker(t, server.URL, &now).Latest(ctx); err == nil || time.Since(started) > 2*time.Second {
		t.Fatalf("Latest() = %v after %v; want a quick timeout error", err, time.Since(started))
	}
}

func TestAvailableNamesOnlyNewerReleases(t *testing.T) {
	var hits atomic.Int32
	server := releaseServer(t, "v0.4.2", &hits)
	now := time.Now()
	checker := newChecker(t, server.URL, &now)

	if got := checker.Available(context.Background(), "0.4.1"); got != "0.4.2" {
		t.Fatalf("Available(0.4.1) = %q, want 0.4.2", got)
	}
	if got := checker.Available(context.Background(), "0.4.2"); got != "" {
		t.Fatalf("Available(0.4.2) = %q, want nothing when up to date", got)
	}
	if got := checker.Available(context.Background(), "dev"); got != "" || hits.Load() != 1 {
		t.Fatalf("Available(dev) = %q with %d requests; want no notice and no extra request", got, hits.Load())
	}
}
