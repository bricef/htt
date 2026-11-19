package gui

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// request sends method host/path to s's handler on testPort.
func request(t *testing.T, s *Server, method, host, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://"+host+path, nil)
	w := httptest.NewRecorder()
	s.Handler(testPort).ServeHTTP(w, r)
	return w
}

func TestServer_Paths(t *testing.T) {
	// The page at the root, the API under /api/, the assets at their
	// paths; nothing else.
	s := testServer(t)
	host := "127.0.0.1:8123"
	cases := []struct {
		path string
		code int
	}{
		{"/", http.StatusOK},
		{"/index.html", http.StatusOK},
		{"/app.js", http.StatusOK},
		{"/api/week", http.StatusOK},
		{"/api/week?date=2026-11-04", http.StatusOK},
		{"/api/month", http.StatusOK},
		{"/api/month?month=2026-10", http.StatusOK},
		{"/api", http.StatusNotFound},
		{"/api/", http.StatusNotFound},
		{"/api/weeks", http.StatusNotFound},
		{"/api/week/", http.StatusNotFound},
		{"/favicon.ico", http.StatusNotFound},
		{"/0123456789abcdef0123456789abcdef/", http.StatusNotFound}, // where a token was
		{"/0123456789abcdef0123456789abcdef/api/week", http.StatusNotFound},
		{"/../api/week", http.StatusNotFound}, // not a clean path
		{"//api/week", http.StatusNotFound},
		{"/./api/week", http.StatusNotFound},
		{"/x/../api/week", http.StatusNotFound},
	}
	for _, c := range cases {
		w := request(t, s, http.MethodGet, host, c.path)
		if w.Code != c.code {
			t.Errorf("GET %s: %d, want %d", c.path, w.Code, c.code)
		}
		if c.code == http.StatusNotFound && strings.Contains(w.Body.String(), `"period"`) {
			t.Errorf("GET %s answered with a view: %s", c.path, w.Body)
		}
	}
	// The page and the API answer at the root as they should: the page
	// with its scripts, the API with its view.
	if body := request(t, s, http.MethodGet, host, "/").Body.String(); !strings.Contains(body, `<script src="app.js"`) {
		t.Errorf("GET /: not the page\n%.200s", body)
	}
	for _, path := range []string{"/api/week", "/api/month"} {
		if body := request(t, s, http.MethodGet, host, path).Body.String(); !strings.Contains(body, `"period"`) {
			t.Errorf("GET %s: not a view\n%.200s", path, body)
		}
	}
	// Another path under api/ says it's not an API.
	if w := request(t, s, http.MethodGet, host, "/api/day"); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "no such API") {
		t.Errorf("GET /api/day: %d %s", w.Code, w.Body)
	}
}

func TestServer_HostMustBeLocal(t *testing.T) {
	// DNS rebinding: a page at evil.example whose name now resolves to
	// 127.0.0.1 sends Host: evil.example:8123. Refused.
	s := testServer(t)
	path := "/api/week"
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

func TestServer_HostOnPort80(t *testing.T) {
	// On port 80, the default, a browser leaves the port out of Host.
	s := testServer(t)
	path := "/api/week"
	for host, code := range map[string]int{
		"127.0.0.1":       http.StatusOK,
		"localhost":       http.StatusOK,
		"LocalHost":       http.StatusOK,
		"127.0.0.1:80":    http.StatusOK,
		"localhost:80":    http.StatusOK,
		"evil.example":    http.StatusForbidden,
		"evil.example:80": http.StatusForbidden,
		"127.0.0.1:8123":  http.StatusForbidden,
		"[::1]":           http.StatusForbidden,
	} {
		r := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
		w := httptest.NewRecorder()
		s.Handler(80).ServeHTTP(w, r)
		if w.Code != code {
			t.Errorf("port 80, Host %s: %d, want %d", host, w.Code, code)
		}
	}
}

func TestServer_OnlyItsOwnPage(t *testing.T) {
	// Only the GUI's own page, or a person opening it, gets an answer:
	// an Origin must be the GUI's own, and a request another site's page
	// makes (Sec-Fetch-Site same-site or cross-site) is refused unless
	// it's a link followed to open the page. Each case as a browser
	// sends it, with a few that isolate one rule.
	s := testServer(t)
	type headers map[string]string
	nav := func(site string) headers {
		return headers{"Sec-Fetch-Site": site, "Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "document"}
	}
	cases := []struct {
		name         string
		port         int
		host, method string
		path         string
		headers      headers
		want         int
	}{
		{"same-origin fetch, its own Origin", 0, "", "", "/api/week",
			headers{"Origin": "http://127.0.0.1:8123", "Sec-Fetch-Site": "same-origin", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty"}, http.StatusOK},
		{"same-origin fetch, no Origin", 0, "", "", "/api/week",
			headers{"Sec-Fetch-Site": "same-origin", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty"}, http.StatusOK},
		{"same-origin script", 0, "", "", "/app.js",
			headers{"Sec-Fetch-Site": "same-origin", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "script"}, http.StatusOK},
		{"none: an address typed, a bookmark, htt's opener", 0, "", "", "/", nav("none"), http.StatusOK},
		{"none, not a navigation", 0, "", http.MethodHead, "/api/week",
			headers{"Sec-Fetch-Site": "none", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "empty"}, http.StatusOK},
		{"no fetch metadata (curl)", 0, "", "", "/api/week", nil, http.StatusOK},
		{"cross-site top-level navigation", 0, "", "", "/", nav("cross-site"), http.StatusOK},
		{"same-site top-level navigation", 0, "", "", "/", nav("same-site"), http.StatusOK},
		{"cross-site fetch (mode cors)", 0, "", "", "/api/week",
			headers{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty"}, http.StatusForbidden},
		{"cross-site fetch without an Origin (mode no-cors)", 0, "", "", "/api/week",
			headers{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "empty"}, http.StatusForbidden},
		{"cross-site no-cors image", 0, "", "", "/icon.svg",
			headers{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "image"}, http.StatusForbidden},
		{"cross-site script", 0, "", "", "/app.js",
			headers{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "script"}, http.StatusForbidden},
		{"cross-site style", 0, "", "", "/style.css",
			headers{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "style"}, http.StatusForbidden},
		{"cross-site iframe", 0, "", "", "/",
			headers{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "iframe"}, http.StatusForbidden},
		{"cross-site object", 0, "", "", "/",
			headers{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "object"}, http.StatusForbidden},
		{"cross-site, a document but not a navigation", 0, "", "", "/",
			headers{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "document"}, http.StatusForbidden},
		{"cross-site form POST, a navigation", 0, "", http.MethodPost, "/api/week", nav("cross-site"), http.StatusForbidden},
		{"cross-site HEAD navigation", 0, "", http.MethodHead, "/", nav("cross-site"), http.StatusForbidden},
		{"same-site fetch (another localhost port)", 0, "", "", "/api/week",
			headers{"Origin": "http://localhost:9000", "Sec-Fetch-Site": "same-site", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty"}, http.StatusForbidden},
		{"same-site image (another localhost port)", 0, "", "", "/icon.svg",
			headers{"Sec-Fetch-Site": "same-site", "Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "image"}, http.StatusForbidden},
		{"a Sec-Fetch-Site no browser sends", 0, "", "", "/api/week", headers{"Sec-Fetch-Site": "same-origin, cross-site"}, http.StatusForbidden},
		{"a foreign Origin, no fetch metadata", 0, "", "", "/api/week", headers{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"a foreign Origin, same-origin claimed", 0, "", "", "/api/week",
			headers{"Origin": "https://evil.example", "Sec-Fetch-Site": "same-origin"}, http.StatusForbidden},
		{"Origin: null (a sandboxed frame, a file)", 0, "", "", "/api/week", headers{"Origin": "null"}, http.StatusForbidden},
		{"an empty Origin", 0, "", "", "/api/week", headers{"Origin": ""}, http.StatusForbidden},
		{"another port's Origin", 0, "", "", "/api/week", headers{"Origin": "http://127.0.0.1:8124"}, http.StatusForbidden},
		{"https at its address", 0, "", "", "/api/week", headers{"Origin": "https://127.0.0.1:8123"}, http.StatusForbidden},
		{"an Origin with a path", 0, "", "", "/api/week", headers{"Origin": "http://127.0.0.1:8123/"}, http.StatusForbidden},
		{"its own Origin via localhost", 0, "localhost:8123", "", "/api/week",
			headers{"Origin": "http://localhost:8123", "Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"its own Origin, the other name", 0, "127.0.0.1:8123", "", "/api/week", headers{"Origin": "http://localhost:8123"}, http.StatusOK},
		{"port 80, the bare host's Origin", 80, "127.0.0.1", "", "/api/week",
			headers{"Origin": "http://127.0.0.1", "Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"port 80, localhost's", 80, "localhost", "", "/api/week", headers{"Origin": "http://localhost"}, http.StatusOK},
		{"port 80, another port's Origin", 80, "127.0.0.1", "", "/api/week", headers{"Origin": "http://127.0.0.1:8123"}, http.StatusForbidden},
	}
	for _, c := range cases {
		port, host, method := c.port, c.host, c.method
		if port == 0 {
			port = testPort
		}
		if host == "" {
			host = "127.0.0.1:8123"
		}
		if method == "" {
			method = http.MethodGet
		}
		r := httptest.NewRequest(method, "http://"+host+c.path, nil)
		for k, v := range c.headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		s.Handler(port).ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s: %d, want %d (%.60q)", c.name, w.Code, c.want, w.Body.String())
			continue
		}
		if c.want == http.StatusForbidden {
			if !strings.HasPrefix(w.Body.String(), "htt gui answers its own page only") {
				t.Errorf("%s: says %q", c.name, w.Body)
			}
			checkSecurityHeaders(t, c.name, w)
		}
	}

	// Two of a header where browsers send one: refused, whichever comes
	// first.
	for _, twice := range [][3]string{
		{"Origin", "http://127.0.0.1:8123", "https://evil.example"},
		{"Sec-Fetch-Site", "same-origin", "cross-site"},
	} {
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8123/api/week", nil)
		r.Header.Add(twice[0], twice[1])
		r.Header.Add(twice[0], twice[2])
		w := httptest.NewRecorder()
		s.Handler(testPort).ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s twice: %d, want 403", twice[0], w.Code)
		}
	}
}

func TestServer_OnlyReads(t *testing.T) {
	s := testServer(t)
	host := "127.0.0.1:8123"
	for _, path := range []string{"/", "/api/week", "/style.css", "/nothing.js", "/api/day"} {
		for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, "PROPFIND"} {
			w := request(t, s, m, host, path)
			if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("%s %s: %d (Allow %q), want 405", m, path, w.Code, w.Header().Get("Allow"))
			}
		}
	}
	// HEAD answers as GET does, without the body.
	get := request(t, s, http.MethodGet, host, "/api/week")
	head := request(t, s, http.MethodHead, host, "/api/week")
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") {
		t.Errorf("HEAD: %d, %d bytes, length %s; GET's %s", head.Code, head.Body.Len(), head.Header().Get("Content-Length"), get.Header().Get("Content-Length"))
	}
}

// wantCSP is the Content-Security-Policy in full: the page's own files
// only, styles inline too (the tooltips set them), and no script but
// its own.
const wantCSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// checkSecurityHeaders fails t unless w carries the headers every
// answer does.
func checkSecurityHeaders(t *testing.T, name string, w *httptest.ResponseRecorder) {
	t.Helper()
	h := w.Header()
	if got := h.Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("%s: Content-Security-Policy\n got %s\nwant %s", name, got, wantCSP)
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("X-Frame-Options") != "DENY" {
		t.Errorf("%s: headers %v", name, h)
	}
}

func TestServer_Headers(t *testing.T) {
	s := testServer(t)
	api := get(t, s, "api/week")
	page := get(t, s, "")
	for name, w := range map[string]*httptest.ResponseRecorder{"api": api, "page": page, "404": get(t, s, "nothing.js")} {
		checkSecurityHeaders(t, name, w)
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
	for _, path := range []string{"vendor/", "vendor", "vendor/plex", "nothing.js", "api", "../go.mod", "%2e%2e/assets.go"} {
		if w := get(t, s, path); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want a 404", path, w.Code)
		}
	}
	for path, want := range map[string]string{
		"app.js":                        "text/javascript; charset=utf-8",
		"style.css":                     "text/css; charset=utf-8",
		"vendor/echarts/echarts.min.js": "text/javascript; charset=utf-8",
		"vendor/plex/IBMPlexMono-Medium-Latin1.woff2": "font/woff2",
		"vendor/echarts/LICENSE":                      "text/plain; charset=utf-8",
		"vendor/README.md":                            "text/plain; charset=utf-8",
	} {
		if w := get(t, s, path); w.Code != http.StatusOK || w.Header().Get("Content-Type") != want || w.Body.Len() == 0 {
			t.Errorf("%s: %d %q, %d bytes; want %q", path, w.Code, w.Header().Get("Content-Type"), w.Body.Len(), want)
		}
	}
}

// TestPage_OnlyItsOwnAssets checks that the page asks for nothing from
// the network, and that every file it asks for is embedded: the
// src and href attributes of index.html and the url()s of style.css.
func TestPage_OnlyItsOwnAssets(t *testing.T) {
	s := testServer(t)
	refs := regexp.MustCompile(`(?:src|href)="([^"#]+)"|url\("([^"]+)"\)`)
	for _, file := range []string{"index.html", "style.css", "app.js"} {
		body := get(t, s, file).Body.String()
		if strings.Contains(body, "://") || strings.Contains(body, `"//`) {
			t.Errorf("%s names another site", file)
		}
		for _, m := range refs.FindAllStringSubmatch(body, -1) {
			ref := m[1] + m[2] // relative to the page; style.css is beside it
			if w := get(t, s, ref); w.Code != http.StatusOK {
				t.Errorf("%s asks for %s: %d", file, ref, w.Code)
			}
		}
	}
	if n := len(refs.FindAllString(get(t, s, "").Body.String(), -1)); n < 3 {
		t.Errorf("index.html: %d assets found, want its stylesheet and two scripts", n)
	}
}

// TestPage_DeclaresItsIcon checks that the page names its own icon. A
// page without one makes Chrome ask for /favicon.ico, which isn't
// there: a 404, and an error in the console.
func TestPage_DeclaresItsIcon(t *testing.T) {
	s := testServer(t)
	m := regexp.MustCompile(`<link rel="icon" href="([^"]+)"`).FindStringSubmatch(get(t, s, "").Body.String())
	if m == nil {
		t.Fatal(`index.html has no <link rel="icon" href="…">`)
	}
	if w := get(t, s, m[1]); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/svg+xml" {
		t.Errorf("the icon %s: %d %s, want an SVG", m[1], w.Code, w.Header().Get("Content-Type"))
	}
}

// blockingRepo is a repository whose Day, once asked, waits until
// release is closed: a request under way for as long as a test likes.
type blockingRepo struct {
	domain.TimelogRepository
	asked    chan struct{} // closed when a request first asks for a day
	release  chan struct{}
	ask, end sync.Once
}

func newBlockingRepo(t *testing.T) *blockingRepo {
	r := &blockingRepo{TimelogRepository: testRepo(t, testZone(t), testLogs), asked: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(r.free)
	return r
}

func (r *blockingRepo) Day(date time.Time) (*domain.Timelog, error) {
	r.ask.Do(func() { close(r.asked) })
	<-r.release
	return r.TimelogRepository.Day(date)
}

// free lets the requests waiting on r go on.
func (r *blockingRepo) free() { r.end.Do(func() { close(r.release) }) }

// serve runs s.ListenAndServe on a free port and waits for its address.
// stop asks it to stop, as Ctrl-C does; done has what it returns.
func serve(t *testing.T, s *Server) (url string, stop func(), done <-chan error) {
	t.Helper()
	signals := make(chan os.Signal, 2)
	urls := make(chan string, 1)
	served := make(chan error, 1)
	go func() { served <- s.ListenAndServe(signals, 0, func(url string) { urls <- url }) }()
	stop = func() { signals <- os.Interrupt }
	select {
	case url = <-urls:
	case err := <-served:
		t.Fatalf("ListenAndServe: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("no address after 10s")
	}
	return url, stop, served
}

// hostOf is the host and port of url, an address htt gui printed.
func hostOf(url string) string {
	return strings.TrimPrefix(strings.TrimSuffix(url, "/"), "http://")
}

// stopped waits for ListenAndServe to return within limit, and says
// whether it returned nil.
func stopped(t *testing.T, done <-chan error, limit time.Duration) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ListenAndServe returned %v once stopped, want nil", err)
		}
	case <-time.After(limit):
		t.Fatalf("still serving %v after it was stopped", limit)
	}
}

func TestListenAndServe(t *testing.T) {
	// On 127.0.0.1, a free port; the address, the page's, at the root;
	// stopping when asked, letting a request under way finish.
	repo := newBlockingRepo(t)
	s := New(Config{Timelogs: repo, Categories: testCategories(t), Now: func() time.Time { return testNow(t) }})
	url, stop, done := serve(t, s)
	if !regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+/$`).MatchString(url) {
		t.Fatalf("address %s", url)
	}
	// localhost works as well as 127.0.0.1.
	for _, page := range []string{url, strings.Replace(url, "127.0.0.1", "localhost", 1)} {
		resp, err := http.Get(page)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: %d", page, resp.StatusCode)
		}
	}

	// A request under way when it's stopped: it waits in the repository
	// until the server has stopped listening, then finishes.
	type answer struct {
		code int
		body string
		err  error
	}
	answers := make(chan answer, 1)
	go func() {
		resp, err := http.Get(url + "api/week")
		if err != nil {
			answers <- answer{err: err}
			return
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		answers <- answer{resp.StatusCode, string(body), err}
	}()
	select {
	case <-repo.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("the request didn't reach the repository in 10s")
	}
	stop()
	host := hostOf(url)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		c, err := net.DialTimeout("tcp", host, time.Second)
		if err != nil {
			break // it has stopped listening
		}
		_ = c.Close()
		if time.Now().After(deadline) {
			t.Fatalf("%s still accepts connections 10s after it was stopped", host)
		}
	}
	repo.free()
	select {
	case a := <-answers:
		if a.err != nil || a.code != http.StatusOK || !strings.Contains(a.body, `"kind":"week"`) {
			t.Errorf("the request under way: %d %v %.80s", a.code, a.err, a.body)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the request under way had no answer in 10s")
	}
	stopped(t, done, 10*time.Second)
}

func TestListenAndServe_ClosesWhatsLeft(t *testing.T) {
	// Stopped with a connection that hasn't sent its request and a
	// request that doesn't finish, it waits a moment, closes them, and
	// returns nil.
	repo := newBlockingRepo(t) // freed when the test ends
	s := New(Config{Timelogs: repo, Categories: testCategories(t), Now: func() time.Time { return testNow(t) }})
	url, stop, done := serve(t, s)
	c, err := net.Dial("tcp", hostOf(url))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := c.Write([]byte("GET /")); err != nil {
		t.Fatal(err)
	}
	go func() {
		if resp, err := http.Get(url + "api/week"); err == nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-repo.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("the request didn't reach the repository in 10s")
	}
	start := time.Now()
	stop()
	stopped(t, done, 10*time.Second)
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("it took %v to stop, want a moment", took.Round(time.Millisecond))
	}
	// The half-sent request's connection is closed, not left open.
	if err := c.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var timeout net.Error
	if _, err := c.Read(make([]byte, 1)); errors.As(err, &timeout) && timeout.Timeout() {
		t.Error("the connection that hadn't sent its request is still open after stopping")
	}
}

func TestListenAndServe_StopsAtOnceWhenAskedTwice(t *testing.T) {
	// A second Ctrl-C doesn't wait for the request under way.
	repo := newBlockingRepo(t) // freed when the test ends
	s := New(Config{Timelogs: repo, Categories: testCategories(t), Now: func() time.Time { return testNow(t) }})
	s.grace = time.Minute
	url, stop, done := serve(t, s)
	go func() {
		if resp, err := http.Get(url + "api/week"); err == nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-repo.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("the request didn't reach the repository in 10s")
	}
	stop()
	stop()
	stopped(t, done, 10*time.Second)
}

func TestListenAndServe_PortTaken(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	port := l.Addr().(*net.TCPAddr).Port
	err = testServer(t).ListenAndServe(nil, port, func(string) { t.Error("ready on a port that was taken") })
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:") {
		t.Errorf("err = %v, want one naming the address", err)
	}
}
