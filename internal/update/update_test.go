package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// defaultReleasesURL is releasesURL before any test points it elsewhere.
var defaultReleasesURL = releasesURL

const releasePath = "/repos/bricef/htt/releases/latest"

var day0 = time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)

// stub serves GitHub's latest-release endpoint from handler, points
// releasesURL at it for the test, and counts the requests it gets.
func stub(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	prev := releasesURL
	releasesURL = srv.URL + releasePath
	t.Cleanup(func() { releasesURL = prev })
	return srv, &hits
}

func releaseJSON(tag string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tag_name": %q, "html_url": "https://github.com/bricef/htt/releases/tag/%s", "draft": false}`, tag, tag)
	}
}

func statePath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "htt", "update-check.json")
}

// finish starts a check and waits for it.
func finish(t *testing.T, cfg Config) *Check {
	t.Helper()
	c := Start(cfg)
	if !c.Wait(5 * time.Second) {
		t.Fatalf("check didn't finish")
	}
	return c
}

func TestDefaultURLIsGitHubsReleaseEndpoint(t *testing.T) {
	// The README promises that the update check contacts this URL and
	// nothing else.
	if defaultReleasesURL != "https://api.github.com/repos/bricef/htt/releases/latest" {
		t.Errorf("releasesURL = %q, want GitHub's latest-release endpoint for bricef/htt", defaultReleasesURL)
	}
}

func TestNewer_ComparesVersions(t *testing.T) {
	cases := []struct {
		current, latest string
		newer           bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.1.0", "v0.1.1", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.3.0", "v0.2.0", false},
		// Pseudo-versions: before the first release, and between releases.
		{"v0.0.0-20261103100930-bfa719f3d777", "v0.1.0", true},
		{"v0.1.1-0.20261103100930-bfa719f3d777+dirty", "v0.1.0", false},
		{"v0.1.1-0.20261103100930-bfa719f3d777", "v0.2.0", true},
	}
	for _, tc := range cases {
		t.Run(tc.current+"_vs_"+tc.latest, func(t *testing.T) {
			stub(t, releaseJSON(tc.latest))
			c := finish(t, Config{Current: tc.current, StatePath: statePath(t), Now: func() time.Time { return day0 }})
			r, ok := c.Newer()
			if ok != tc.newer {
				t.Fatalf("Newer() = %v, want %v (err %v)", ok, tc.newer, c.Err())
			}
			if ok && (r.Version != tc.latest || r.Current != tc.current) {
				t.Errorf("Release = %+v", r)
			}
		})
	}
}

func TestNotice_SaysWhatToDo(t *testing.T) {
	r := Release{Version: "v0.2.0", Current: "v0.1.0", URL: "https://github.com/bricef/htt/releases/tag/v0.2.0"}
	for _, want := range []string{"v0.2.0", "you have v0.1.0", "go install github.com/bricef/htt/cmd/htt@latest", r.URL, "check_for_updates: false"} {
		if !strings.Contains(r.Notice(), want) {
			t.Errorf("Notice() missing %q:\n%s", want, r.Notice())
		}
	}
	if short := r.Short(); strings.Contains(short, "\n") || !strings.Contains(short, "v0.2.0") {
		t.Errorf("Short() = %q, want one line naming the release", short)
	}
}

func TestStart_SendsAPlainRequestToTheReleaseEndpoint(t *testing.T) {
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != releasePath || r.URL.RawQuery != "" {
			t.Errorf("request = %s %s?%s, want GET %s", r.Method, r.URL.Path, r.URL.RawQuery, releasePath)
		}
		if got := r.Header.Get("User-Agent"); got != "htt/v0.1.0" {
			t.Errorf("User-Agent = %q, want htt/v0.1.0", got)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q", got)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Errorf("the check must not send credentials")
		}
		releaseJSON("v0.1.0")(w, r)
	})
	c := finish(t, Config{Current: "v0.1.0", StatePath: statePath(t), Now: func() time.Time { return day0 }})
	if err := c.Err(); err != nil {
		t.Errorf("Err() = %v", err)
	}
}

func TestStart_AtMostOnceADay(t *testing.T) {
	_, hits := stub(t, releaseJSON("v0.2.0"))
	path := statePath(t)
	at := func(d time.Duration) Config {
		return Config{Current: "v0.1.0", StatePath: path, Now: func() time.Time { return day0.Add(d) }}
	}

	if _, ok := finish(t, at(0)).Newer(); !ok {
		t.Fatalf("first check should find v0.2.0")
	}
	// Later the same day: no request, and no repeat of the notice.
	c := finish(t, at(23*time.Hour))
	if _, ok := c.Newer(); ok || hits.Load() != 1 {
		t.Errorf("same-day run: Newer=%v hits=%d, want no notice and no request", ok, hits.Load())
	}
	// A day later: asks again.
	if _, ok := finish(t, at(24*time.Hour)).Newer(); !ok || hits.Load() != 2 {
		t.Errorf("next-day run: hits=%d, want a second request and a notice", hits.Load())
	}
	// A clock set back past the last check counts as due.
	finish(t, at(-time.Hour))
	if hits.Load() != 3 {
		t.Errorf("clock set back: hits=%d, want a request", hits.Load())
	}
}

func TestStart_RecordsTheAttemptBeforeAsking(t *testing.T) {
	path := statePath(t)
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		if got := loadState(path).CheckedAt; !got.Equal(day0) {
			t.Errorf("state when the request arrived: checked_at = %v, want %v already recorded", got, day0)
		}
		releaseJSON("v0.2.0")(w, r)
	})
	finish(t, Config{Current: "v0.1.0", StatePath: path, Now: func() time.Time { return day0 }})
}

func TestStart_WithoutAVersionNeverAsks(t *testing.T) {
	_, hits := stub(t, releaseJSON("v0.2.0"))
	for _, current := range []string{"", "(devel)", "not-a-version"} {
		c := finish(t, Config{Current: current, StatePath: statePath(t)})
		if _, ok := c.Newer(); ok {
			t.Errorf("Current %q: unexpected notice", current)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("hits = %d, want no request for unversioned builds", hits.Load())
	}
}

func TestStart_NoReleaseYetIsNotAnError(t *testing.T) {
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := finish(t, Config{Current: "v0.1.0", StatePath: statePath(t), Now: func() time.Time { return day0 }})
	if _, ok := c.Newer(); ok || c.Err() != nil {
		t.Errorf("404: Newer=%v Err=%v, want no notice and no error", ok, c.Err())
	}
}

func TestStart_FailuresAreSilent(t *testing.T) {
	prevTimeout := requestTimeout
	requestTimeout = 200 * time.Millisecond
	t.Cleanup(func() { requestTimeout = prevTimeout })

	cases := []struct {
		name    string
		handler http.HandlerFunc
		closed  bool // the server is gone: connection refused
	}{
		{name: "offline", closed: true},
		{name: "timeout", handler: func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
		}},
		{name: "rate limited", handler: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
		}},
		{name: "server error", handler: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}},
		{name: "malformed JSON", handler: func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "<html>") }},
		{name: "tag isn't a version", handler: releaseJSON("latest")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := tc.handler
			if handler == nil {
				handler = releaseJSON("v9.9.9")
			}
			srv, _ := stub(t, handler)
			if tc.closed {
				srv.Close()
			}
			path := statePath(t)
			start := time.Now()
			c := finish(t, Config{Current: "v0.1.0", StatePath: path, Now: func() time.Time { return day0 }})
			if _, ok := c.Newer(); ok {
				t.Errorf("unexpected notice")
			}
			if c.Err() == nil {
				t.Errorf("Err() = nil, want the reason for diagnostics")
			}
			if time.Since(start) > 2*time.Second {
				t.Errorf("took %v, want the request timeout to bound it", time.Since(start))
			}
			// The attempt still counts, so the next run doesn't retry.
			if got := loadState(path).CheckedAt; !got.Equal(day0) {
				t.Errorf("checked_at = %v, want the failed attempt recorded", got)
			}
		})
	}
}

func TestStart_RefusesRedirectsOffTheHost(t *testing.T) {
	var elsewhere atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere.Add(1)
		releaseJSON("v9.9.9")(w, r)
	}))
	t.Cleanup(other.Close)
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		// httptest servers share 127.0.0.1, so use the hostname form to
		// make the redirect target a different host.
		target := strings.Replace(other.URL, "127.0.0.1", "localhost", 1) + releasePath
		http.Redirect(w, r, target, http.StatusFound)
	})
	c := finish(t, Config{Current: "v0.1.0", StatePath: statePath(t), Now: func() time.Time { return day0 }})
	if elsewhere.Load() != 0 {
		t.Errorf("the check followed a redirect to another host")
	}
	if c.Err() == nil || !strings.Contains(c.Err().Error(), "refusing redirect") {
		t.Errorf("Err() = %v, want a refused redirect", c.Err())
	}
}

func TestStart_UnwritableCacheMeansNoCheck(t *testing.T) {
	_, hits := stub(t, releaseJSON("v0.2.0"))
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", filepath.Join(blocker, "update-check.json")} {
		c := finish(t, Config{Current: "v0.1.0", StatePath: path, Now: func() time.Time { return day0 }})
		if _, ok := c.Newer(); ok || c.Err() == nil {
			t.Errorf("path %q: Newer=%v Err=%v, want no notice and a reason", path, ok, c.Err())
		}
	}
	// Without a record of the attempt, every run would ask, so none do.
	if hits.Load() != 0 {
		t.Errorf("hits = %d, want no request when the attempt can't be recorded", hits.Load())
	}
}

func TestStart_CorruptCacheIsTreatedAsNoCache(t *testing.T) {
	_, hits := stub(t, releaseJSON("v0.2.0"))
	path := statePath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := finish(t, Config{Current: "v0.1.0", StatePath: path, Now: func() time.Time { return day0 }}).Newer(); !ok || hits.Load() != 1 {
		t.Errorf("corrupt cache: hits=%d, want a fresh check that finds v0.2.0", hits.Load())
	}
}
