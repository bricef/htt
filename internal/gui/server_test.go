package gui

import (
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

func TestServer_HostOnPort80(t *testing.T) {
	// On port 80, the default, a browser leaves the port out of Host.
	s := testServer(t)
	path := "/" + testToken + "/api/week"
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
	// The policy in full: the page's own files only, styles inline too
	// (the tooltips set them), and no script but its own.
	const csp = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
		"frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	for name, w := range map[string]*httptest.ResponseRecorder{"api": api, "page": page, "404": get(t, s, "nothing.js")} {
		h := w.Header()
		if got := h.Get("Content-Security-Policy"); got != csp {
			t.Errorf("%s: Content-Security-Policy\n got %s\nwant %s", name, got, csp)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("X-Frame-Options") != "DENY" {
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
// page without one makes Chrome ask for /favicon.ico, outside the
// token, which is a 404 and an error in the console.
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
	return strings.TrimPrefix(strings.TrimSuffix(url, "/"+testToken+"/"), "http://")
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
	// On 127.0.0.1, a free port; the address with the token; stopping
	// when asked, letting a request under way finish.
	repo := newBlockingRepo(t)
	s := newServer(Config{Timelogs: repo, Categories: testCategories(t), Now: func() time.Time { return testNow(t) }}, testToken)
	url, stop, done := serve(t, s)
	if !regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+/` + testToken + `/$`).MatchString(url) {
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
	s := newServer(Config{Timelogs: repo, Categories: testCategories(t), Now: func() time.Time { return testNow(t) }}, testToken)
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
}

func TestListenAndServe_StopsAtOnceWhenAskedTwice(t *testing.T) {
	// A second Ctrl-C doesn't wait for the request under way.
	repo := newBlockingRepo(t) // freed when the test ends
	s := newServer(Config{Timelogs: repo, Categories: testCategories(t), Now: func() time.Time { return testNow(t) }}, testToken)
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
