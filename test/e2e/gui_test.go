package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// guiAddress is the line htt gui prints once it serves.
var guiAddress = regexp.MustCompile(`^htt gui: (http://127\.0\.0\.1:[0-9]+/) \(Ctrl-C to stop\)$`)

// guiProcess is a running `htt gui`.
type guiProcess struct {
	cmd    *exec.Cmd
	url    string // the page's address, as printed: http://127.0.0.1:<port>/
	mu     sync.Mutex
	stdout bytes.Buffer // after the address line
	stderr bytes.Buffer
	done   chan error
}

func (g *guiProcess) output() (string, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stdout.String(), g.stderr.String()
}

// syncWriter guards a buffer the process writes to while the test reads.
type syncWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// startGUI runs `htt gui --no-open --port 0` in the env and waits for
// the address it prints. The process is killed when the test ends, if
// it's still running.
func (e *env) startGUI(args ...string) *guiProcess {
	e.t.Helper()
	return e.startGUIWith(nil, append([]string{"--no-open"}, args...)...)
}

// startGUIWith runs `htt gui --port 0` with args, as startGUI does, after
// prepare (if not nil) has set up the command.
func (e *env) startGUIWith(prepare func(*exec.Cmd), args ...string) *guiProcess {
	e.t.Helper()
	g := &guiProcess{done: make(chan error, 1)}
	g.cmd = exec.Command(binaryPath, append([]string{"--no-color", "gui", "--port", "0"}, args...)...)
	g.cmd.Env = append(append(e.homeEnv(), "PATH="+os.Getenv("PATH")), e.extraEnv...)
	g.cmd.Stderr = syncWriter{&g.mu, &g.stderr}
	if prepare != nil {
		prepare(g.cmd)
	}
	stdout, err := g.cmd.StdoutPipe()
	if err != nil {
		e.t.Fatal(err)
	}
	if err := g.cmd.Start(); err != nil {
		e.t.Fatalf("start htt gui: %v", err)
	}
	e.t.Cleanup(func() {
		_ = g.cmd.Process.Kill()
		if err := <-g.done; err != nil { // killed, or it exited by itself
			e.t.Logf("htt gui: %v", err)
		}
	})
	lines := make(chan string)
	go func() {
		r := bufio.NewReader(stdout)
		line, _ := r.ReadString('\n')
		lines <- strings.TrimRight(line, "\r\n")
		_, _ = io.Copy(syncWriter{&g.mu, &g.stdout}, r)
		g.done <- g.cmd.Wait()
	}()
	select {
	case line := <-lines:
		m := guiAddress.FindStringSubmatch(line)
		if m == nil {
			_, stderr := g.output()
			e.t.Fatalf("htt gui printed %q, want its address\nstderr: %s", line, stderr)
		}
		g.url = m[1]
	case <-time.After(20 * time.Second):
		e.t.Fatal("htt gui printed no address in 20s")
	}
	return g
}

// fetch GETs url, with Host host when it isn't "", and returns the
// status and body.
func fetch(t *testing.T, method, url, host string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

// guiView is the part of the API's answer these tests read.
type guiView struct {
	Version    int
	Error      string
	Categories []struct {
		Name string
		Work bool
	}
	Period struct {
		Kind, Key, Start, End string
	}
	Days []struct {
		Date, Status       string
		Worked, ToMidnight int64
		Breaks             int64
		Categories         map[string]int64
		Intervals          []struct {
			Entry, Start, Open string
			StartMinute        float64
		}
	}
	Summary struct {
		Unended []string
	}
}

func getView(t *testing.T, url string) guiView {
	t.Helper()
	code, body := fetch(t, http.MethodGet, url, "")
	var v guiView
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("GET %s: %d %v\n%s", url, code, err, body)
	}
	if code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", url, code, v.Error)
	}
	return v
}

const guiCategories = `timelog:
  categories:
    - name: break
      tags: ["@lunch"]
      work: false
    - name: htt
      tags: ["+htt"]
check_for_updates: false
`

func TestGUI_ServesTheTimeLog(t *testing.T) {
	// htt in Asia/Kolkata (+05:30): the week of 2 November 2026 from
	// its logs, a day ended and one without @end, in categories; and
	// the month that holds it.
	e, loc := timelogEnv(t)
	e.writeConfig(guiCategories)
	e.writeTimelog(loc, "2026-11-02", "09:00 fix +htt", "12:00 @lunch", "13:00 review +htt", "17:00 @end")
	e.writeTimelog(loc, "2026-11-03", "09:30 answer mail", "10:00 fix +htt")
	g := e.startGUI()

	code, page := fetch(t, http.MethodGet, g.url, "")
	if code != http.StatusOK || !strings.Contains(string(page), "app.js") {
		t.Errorf("the page: %d\n%s", code, page)
	}

	v := getView(t, g.url+"api/week?date=2026-11-04")
	if v.Version != 1 || v.Period.Kind != "week" || v.Period.Key != "2026-11-02" || len(v.Days) != 7 {
		t.Fatalf("week: version %d, period %+v, %d days", v.Version, v.Period, len(v.Days))
	}
	var names []string
	for _, c := range v.Categories {
		names = append(names, c.Name)
	}
	assertEqual(t, "categories", strings.Join(names, " "), "break htt uncategorised")
	mon, tue := v.Days[0], v.Days[1]
	if mon.Status != "ended" || mon.Worked != 7*3600 || mon.Breaks != 3600 || mon.Categories["htt"] != 7*3600 {
		t.Errorf("Monday: %+v", mon)
	}
	if first := mon.Intervals[0]; first.Entry != "fix +htt" || first.StartMinute != 540 || first.Start != "2026-11-02T09:00:00+05:30" {
		t.Errorf("Monday's first entry: %+v", first)
	}
	if tue.Status != "unended" || tue.ToMidnight != 14*3600 || tue.Categories["uncategorised"] != 1800 ||
		tue.Intervals[len(tue.Intervals)-1].Open != "midnight" {
		t.Errorf("Tuesday: %+v", tue)
	}

	m := getView(t, g.url+"api/month?month=2026-11")
	if m.Period.Kind != "month" || m.Period.Start != "2026-11-01" || m.Period.End != "2026-11-30" || len(m.Days) != 30 {
		t.Fatalf("month: period %+v, %d days", m.Period, len(m.Days))
	}
	if m.Days[1].Worked != 7*3600 || len(m.Summary.Unended) == 0 || m.Summary.Unended[0] != "2026-11-03" {
		t.Errorf("month: the 2nd worked %d, unended %v", m.Days[1].Worked, m.Summary.Unended)
	}

	if code, _ := fetch(t, http.MethodGet, g.url+"api/week?date=2026-11-31", ""); code != http.StatusBadRequest {
		t.Errorf("a date that doesn't exist: %d, want 400", code)
	}
}

func TestGUI_GuardsAndStopsOnInterrupt(t *testing.T) {
	// Runs everywhere, Windows too: the paths, the Host and the methods,
	// then Ctrl-C. Today's week, whatever today is.
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	g := e.startGUI()

	if v := getView(t, g.url+"api/week"); v.Period.Kind != "week" || len(v.Days) != 7 {
		t.Errorf("this week: %+v", v.Period)
	}
	for _, path := range []string{"api/day", "nothing.js", "0123456789abcdef0123456789abcdef/"} {
		if code, _ := fetch(t, http.MethodGet, g.url+path, ""); code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", g.url+path, code)
		}
	}
	host := strings.TrimSuffix(strings.TrimPrefix(g.url, "http://"), "/")
	port := host[strings.LastIndex(host, ":"):]
	if code, _ := fetch(t, http.MethodGet, g.url+"api/week", "evil.example"+port); code != http.StatusForbidden {
		t.Errorf("Host evil.example%s: %d, want 403", port, code)
	}
	if code, _ := fetch(t, http.MethodGet, g.url+"api/week", "localhost"+port); code != http.StatusOK {
		t.Errorf("Host localhost%s: %d, want 200", port, code)
	}
	if code, _ := fetch(t, http.MethodPost, g.url+"api/week", ""); code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d, want 405", code)
	}

	if runtime.GOOS == "windows" {
		// Go can't send another process Ctrl-C on Windows:
		// os.Process.Signal supports only Kill there.
		t.Skip("stopping htt gui with an interrupt: Go can't send one to another process on Windows")
	}
	if err := g.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-g.done:
		g.done <- err // for the cleanup
		stdout, stderr := g.output()
		if err != nil {
			t.Errorf("htt gui exited with %v after an interrupt\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		if stderr != "" {
			t.Errorf("htt gui wrote to stderr: %s", stderr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("htt gui still running 10s after an interrupt")
	}
	if _, err := (&http.Client{Timeout: 2 * time.Second}).Get(g.url); err == nil {
		t.Errorf("still answering after it stopped")
	}
}

func TestGUI_StopsAtOnceWithAConnectionOpen(t *testing.T) {
	// A browser holds connections open, and one may not have sent its
	// request yet: Ctrl-C stops htt gui all the same, within a moment,
	// and cleanly.
	if runtime.GOOS == "windows" {
		t.Skip("stopping htt gui with an interrupt: Go can't send one to another process on Windows")
	}
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	g := e.startGUI()
	host := strings.TrimPrefix(g.url, "http://")
	c, err := net.Dial("tcp", host[:strings.Index(host, "/")])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := c.Write([]byte("GET /")); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := g.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-g.done:
		g.done <- err // for the cleanup
		stdout, stderr := g.output()
		if err != nil || stderr != "" {
			t.Errorf("htt gui exited with %v after an interrupt\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		if took := time.Since(start); took > 3*time.Second {
			t.Errorf("htt gui took %v to stop, want a moment", took.Round(time.Millisecond))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("htt gui still running 10s after an interrupt")
	}
}

func TestGUI_ShowsAConfigItCantUse(t *testing.T) {
	// The config's error, naming the key, in the page's API and on
	// stderr; the server still runs, for the page to show it.
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\ntimelog:\n  categories:\n    - name: htt\n      tags: [\"htt\"]\n")
	g := e.startGUI()
	code, body := fetch(t, http.MethodGet, g.url+"api/week", "")
	if code != http.StatusInternalServerError {
		t.Errorf("api/week: %d, want 500", code)
	}
	assertContains(t, "API error", string(body), `timelog.categories: category 1 (htt): \"htt\" isn't a tag`)
	assertContains(t, "API error", string(body), "restart htt gui")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, stderr := g.output(); strings.Contains(stderr, "warning: config: timelog.categories") || time.Now().After(deadline) {
			assertContains(t, "stderr", stderr, "warning: config: timelog.categories: category 1 (htt)")
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestGUI_PortErrors(t *testing.T) {
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	r := e.run("gui", "--no-open", "--port", "70000")
	if r.exitCode != 1 {
		t.Errorf("--port 70000: exit %d, want 1", r.exitCode)
	}
	assertContains(t, "stderr", r.stderr, "--port: want 0 to 65535")

	// A port already taken: the error names the address.
	g := e.startGUI()
	host := strings.TrimPrefix(g.url, "http://")
	port := host[strings.Index(host, ":")+1 : strings.Index(host, "/")]
	r = e.run("gui", "--no-open", "--port", port)
	if r.exitCode != 1 {
		t.Errorf("--port %s, taken: exit %d, want 1", port, r.exitCode)
	}
	assertContains(t, "stderr", r.stderr, "listen on 127.0.0.1:"+port)
}

func TestGUI_NotInTheREPL(t *testing.T) {
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	r := e.mustRunWithInput("/gui --no-open\nq\n", "repl")
	assertContains(t, "stderr", r.stderr, `"gui" is not available inside the REPL`)
}
