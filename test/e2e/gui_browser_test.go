package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// chromePath finds Chrome or Chromium for the browser test:
// $HTT_TEST_CHROME (which can be a script that sets up the browser's
// environment and execs it, say for LD_LIBRARY_PATH), else one on PATH,
// else where it's installed by default on macOS and Windows; "" when
// there's none.
func chromePath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("HTT_TEST_CHROME"); p != "" {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("HTT_TEST_CHROME: %v", err)
		}
		return p
	}
	for _, name := range []string{"chrome-headless-shell", "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"}
	case "windows":
		for _, dir := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
			if dir != "" {
				candidates = append(candidates, filepath.Join(dir, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// TestGUI_InTheBrowser loads the page from the real binary in headless
// Chrome and checks it draws both views, and a known label in each,
// without a JavaScript exception or an error on the console (a failed
// load or a Content-Security-Policy violation included). It's skipped
// where there's no Chrome; HTT_TEST_CHROME names one.
func TestGUI_InTheBrowser(t *testing.T) {
	chrome := chromePath(t)
	if chrome == "" {
		t.Skip("no Chrome or Chromium found; set HTT_TEST_CHROME to run the browser test")
	}
	// The logs are written in the zone htt runs in: pinned with TZ
	// where Go reads it, the system's on Windows.
	e := newEnv(t)
	loc := time.Local
	if runtime.GOOS != "windows" {
		var err error
		if loc, err = time.LoadLocation(timelogZone); err != nil {
			t.Fatal(err)
		}
		e.extraEnv = append(e.extraEnv, "TZ="+timelogZone)
	}
	e.writeConfig(`check_for_updates: false
timelog:
  categories:
    - name: break
      tags: ["@lunch"]
      work: false
    - name: smoke-test
      tags: ["+smoke"]
`)
	e.writeTimelog(loc, "2026-11-02", "09:00 fix +smoke", "12:00 @lunch", "13:00 review +smoke", "17:00 @end")
	e.writeTimelog(loc, "2026-11-03", "09:30 answer mail", "10:00 fix +smoke")
	g := e.startGUI()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chrome),
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.WindowSize(1180, 1400),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	if err := chromedp.Do(ctx); err != nil { // start the browser, with no timeout
		t.Fatalf("start %s: %v", chrome, err)
	}

	// Everything the page says on the console, exceptions and the
	// browser's log (a failed load, a Content-Security-Policy violation)
	// included; the errors fail the test.
	var mu sync.Mutex
	var problems []string
	messages := chromedp.Console(ctx)
	listening, stopListening := context.WithCancel(context.Background())
	go func() {
		for m, err := range messages {
			if err != nil || listening.Err() != nil {
				return
			}
			if m.Type == chromedp.ConsoleError || m.IsException() {
				mu.Lock()
				problems = append(problems, fmt.Sprintf("%s (%s:%d)", m, m.URL, m.Line+1))
				mu.Unlock()
			}
		}
	}()
	defer stopListening()

	tctx, tcancel := context.WithTimeout(ctx, 90*time.Second)
	defer tcancel()
	// waitFor polls the page until the JavaScript condition holds.
	waitFor := func(what, condition string) chromedp.Action[chromedp.Void] {
		return func(ctx context.Context, tg *chromedp.Target) (chromedp.Void, error) {
			if _, err := chromedp.Poll[bool](condition, chromedp.WithPollingTimeout(30*time.Second))(ctx, tg); err != nil {
				return chromedp.Void{}, fmt.Errorf("waiting for %s: %w", what, err)
			}
			return chromedp.Void{}, nil
		}
	}
	// svgText is the text the charts drew in the element.
	svgText := func(id string) string {
		return fmt.Sprintf(`(document.querySelector("#%s svg") || {textContent: ""}).textContent`, id)
	}
	fail := func(err error) {
		t.Helper()
		stdout, stderr := g.output()
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("%v\nthe page reported: %v\nhtt gui stdout: %s\nstderr: %s", err, problems, stdout, stderr)
	}

	// The week: the timeline's legend and labels, the totals' labels;
	// the zoom out to the whole day.
	if err := chromedp.Do(tctx,
		chromedp.Navigate(g.url+"#week/2026-11-04"),
		waitFor("the week's timeline", svgText("timeline")+`.includes("smoke-test") && `+svgText("timeline")+`.includes("no @end")`),
		waitFor("the week's totals", svgText("summary")+`.includes("7h 0m")`),
		waitFor("the week's title", `document.querySelector("#title").textContent === "2–8 November 2026"`),
		chromedp.Click(chromedp.ID("zoom")),
		waitFor("the whole day", `document.querySelector("#zoom").textContent === "Hours logged"`),
	); err != nil {
		fail(err)
	}
	// The month, by its route: the calendar, the charts, the table.
	if err := chromedp.Do(tctx,
		chromedp.Evaluate[chromedp.Void](`location.hash = "#month/2026-11"`),
		waitFor("the month's calendar", svgText("calendar")+`.includes("7h 0m")`),
		waitFor("the month's charts", svgText("trends")+`.includes("smoke-test") && `+svgText("weeks")+`.includes("2–8 Nov") && `+svgText("weekday")+`.includes("Mon")`),
		waitFor("the month's table", `document.querySelector("#month-totals").textContent.includes("smoke-test")`),
		waitFor("the month's title", `document.querySelector("#title").textContent === "November 2026"`),
	); err != nil {
		fail(err)
	}
	// Dark, when the system asks for it, drawn again; then the week by
	// its tab, which goes back to the week shown before.
	if _, err := chromedp.Call(tctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "dark"}},
	}); err != nil {
		fail(err)
	}
	if err := chromedp.Do(tctx,
		waitFor("the dark page", `getComputedStyle(document.body).backgroundColor === "rgb(14, 20, 23)"`),
		waitFor("the dark calendar", svgText("calendar")+`.includes("7h 0m")`),
		chromedp.Click(chromedp.ID("tab-week")),
		waitFor("the week again", `document.querySelector("#title").textContent === "2–8 November 2026" && location.hash === "#week/2026-11-04"`),
		waitFor("the dark week", svgText("timeline")+`.includes("smoke-test")`),
	); err != nil {
		fail(err)
	}

	time.Sleep(200 * time.Millisecond) // the last messages on their way
	stopListening()
	mu.Lock()
	defer mu.Unlock()
	if len(problems) > 0 {
		t.Errorf("the page reported:\n  %s", strings.Join(problems, "\n  "))
	}
}
