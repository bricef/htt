package gui

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// request sends method host/path to s's handler on testPort.
func request(t *testing.T, s *Server, method, host, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://"+host+path, nil)
	w := httptest.NewRecorder()
	s.Handler(testPort).ServeHTTP(w, r)
	return w
}

func TestServer_EveryPathStartsWithTheToken(t *testing.T) {
	s := testServer(t)
	host := "127.0.0.1:8123"
	cases := []struct {
		path string
		code int
	}{
		{"/" + testToken + "/", http.StatusOK},
		{"/" + testToken + "/api/week", http.StatusOK},
		{"/" + testToken + "/api/month?month=2026-10", http.StatusOK},
		{"/" + testToken, http.StatusFound}, // to the page, with its slash
		{"/", http.StatusNotFound},
		{"/api/week", http.StatusNotFound},
		{"/index.html", http.StatusNotFound},
		{"/" + strings.ToUpper(testToken) + "/", http.StatusNotFound},
		{"/" + testToken[:31] + "/", http.StatusNotFound},          // a character short
		{"/" + testToken[:31] + "0/api/week", http.StatusNotFound}, // one wrong
		{"/" + testToken + "x/", http.StatusNotFound},
		{"/x" + testToken + "/", http.StatusNotFound},
		{"/" + testToken + "/../" + testToken + "/api/week", http.StatusNotFound}, // not a clean path
		{"/" + testToken + "//api/week", http.StatusNotFound},
		{"/" + testToken + "/./api/week", http.StatusNotFound},
	}
	for _, c := range cases {
		w := request(t, s, http.MethodGet, host, c.path)
		if w.Code != c.code {
			t.Errorf("GET %s: %d, want %d", c.path, w.Code, c.code)
		}
		if c.code == http.StatusNotFound && strings.Contains(w.Body.String(), "version") {
			t.Errorf("GET %s answered from the API: %s", c.path, w.Body)
		}
	}
	if loc := request(t, s, http.MethodGet, host, "/"+testToken).Header().Get("Location"); loc != "/"+testToken+"/" {
		t.Errorf("the redirect goes to %q", loc)
	}
}

func TestServer_HostMustBeLocal(t *testing.T) {
	// DNS rebinding: a page at evil.example whose name now resolves to
	// 127.0.0.1 sends Host: evil.example:8123. Refused, token or not.
	s := testServer(t)
	path := "/" + testToken + "/api/week"
	for host, code := range map[string]int{
		"127.0.0.1:8123":    http.StatusOK,
		"localhost:8123":    http.StatusOK,
		"LocalHost:8123":    http.StatusOK,
		"evil.example:8123": http.StatusForbidden,
		"127.0.0.1:8124":    http.StatusForbidden,
		"127.0.0.1":         http.StatusForbidden,
		"localhost":         http.StatusForbidden,
		"[::1]:8123":        http.StatusForbidden,
		"0.0.0.0:8123":      http.StatusForbidden,
		"127.0.0.2:8123":    http.StatusForbidden,
		"localhost.:8123":   http.StatusForbidden,
	} {
		if w := request(t, s, http.MethodGet, host, path); w.Code != code {
			t.Errorf("Host %s: %d, want %d", host, w.Code, code)
		}
	}
}

func TestServer_OnlyReads(t *testing.T) {
	s := testServer(t)
	host := "127.0.0.1:8123"
	for _, path := range []string{"/" + testToken + "/", "/" + testToken + "/api/week", "/" + testToken + "/style.css"} {
		for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, "PROPFIND"} {
			w := request(t, s, m, host, path)
			if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("%s %s: %d (Allow %q), want 405", m, path, w.Code, w.Header().Get("Allow"))
			}
		}
	}
	// HEAD answers as GET does, without the body.
	get := request(t, s, http.MethodGet, host, "/"+testToken+"/api/week")
	head := request(t, s, http.MethodHead, host, "/"+testToken+"/api/week")
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") {
		t.Errorf("HEAD: %d, %d bytes, length %s; GET's %s", head.Code, head.Body.Len(), head.Header().Get("Content-Length"), get.Header().Get("Content-Length"))
	}
	// A wrong token is a 404 whatever the method.
	if w := request(t, s, http.MethodPost, host, "/nottoken/api/week"); w.Code != http.StatusNotFound {
		t.Errorf("POST with a wrong token: %d, want 404", w.Code)
	}
}

func TestServer_Headers(t *testing.T) {
	s := testServer(t)
	api := get(t, s, "api/week")
	page := get(t, s, "")
	for name, w := range map[string]*httptest.ResponseRecorder{"api": api, "page": page} {
		h := w.Header()
		if !strings.Contains(h.Get("Content-Security-Policy"), "default-src 'self'") ||
			h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s: headers %v", name, h)
		}
	}
	if api.Header().Get("Cache-Control") != "no-store" || api.Header().Get("Content-Type") != "application/json" {
		t.Errorf("api: headers %v", api.Header())
	}
	if !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
		t.Errorf("page: Content-Type %q", page.Header().Get("Content-Type"))
	}
}

func TestServer_Assets(t *testing.T) {
	s := testServer(t)
	if w := get(t, s, "index.html"); w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("index.html: %d %s, want the page", w.Code, w.Header().Get("Content-Type"))
	}
	for _, path := range []string{"vendor/", "vendor", "nothing.js", "api", "../go.mod", "%2e%2e/assets.go"} {
		if w := get(t, s, path); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want a 404", path, w.Code)
		}
	}
}

func TestNew_RandomTokens(t *testing.T) {
	seen := map[string]bool{}
	for range 20 {
		s, err := New(Config{})
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^/[0-9a-f]{32}/$`).MatchString(s.Path()) || seen[s.Path()] {
			t.Fatalf("path %s: want 128 random bits, new each time", s.Path())
		}
		seen[s.Path()] = true
	}
}

func TestListenAndServe(t *testing.T) {
	// On 127.0.0.1, a free port; the address with the token; stopping
	// when the context is done, letting a request under way finish.
	s := testServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe(ctx, 0, func(url string) { urls <- url }) }()
	var url string
	select {
	case url = <-urls:
	case err := <-done:
		t.Fatalf("ListenAndServe: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("no address after 10s")
	}
	if !regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+/` + testToken + `/$`).MatchString(url) {
		t.Fatalf("address %s", url)
	}
	resp, err := http.Get(url + "api/week")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"kind":"week"`) {
		t.Errorf("GET api/week: %d %s", resp.StatusCode, body)
	}
	// localhost works as well as 127.0.0.1.
	if resp, err := http.Get(strings.Replace(url, "127.0.0.1", "localhost", 1)); err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("localhost: %d", resp.StatusCode)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ListenAndServe returned %v after the context was done, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("still serving 10s after the context was done")
	}
	host := strings.TrimPrefix(strings.TrimSuffix(url, "/"+testToken+"/"), "http://")
	if c, err := net.DialTimeout("tcp", host, time.Second); err == nil {
		_ = c.Close()
		t.Errorf("%s still accepts connections", host)
	}
}

func TestListenAndServe_PortTaken(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	port := l.Addr().(*net.TCPAddr).Port
	err = testServer(t).ListenAndServe(context.Background(), port, func(string) { t.Error("ready on a port that was taken") })
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:") {
		t.Errorf("err = %v, want one naming the address", err)
	}
}
