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

// browser is headless Chrome for a test, and the errors the pages it
// loads report.
type browser struct {
	t        *testing.T
	ctx      context.Context // the test's, with a timeout
	mu       sync.Mutex
	problems []string
}

// newBrowser starts headless Chrome for t, its window width × height,
// and skips t where there's none; HTT_TEST_CHROME names one. It stops
// when t ends.
func newBrowser(t *testing.T, width, height int) *browser {
	t.Helper()
	chrome := chromePath(t)
	if chrome == "" {
		t.Skip("no Chrome or Chromium found; set HTT_TEST_CHROME to run the browser test")
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chrome),
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.WindowSize(width, height),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	t.Cleanup(cancelAlloc)
	ctx, cancel := chromedp.NewContext(allocCtx)
	t.Cleanup(cancel)
	if err := chromedp.Do(ctx); err != nil { // start the browser, with no timeout
		t.Fatalf("start %s: %v", chrome, err)
	}

	// Everything the page says on the console, exceptions and the
	// browser's log (a failed load, a Content-Security-Policy violation)
	// included; the errors fail the test.
	b := &browser{t: t}
	messages := chromedp.Console(ctx)
	listening, stopListening := context.WithCancel(context.Background())
	t.Cleanup(stopListening)
	go func() {
		for m, err := range messages {
			if err != nil || listening.Err() != nil {
				return
			}
			if m.Type == chromedp.ConsoleError || m.IsException() {
				b.mu.Lock()
				b.problems = append(b.problems, fmt.Sprintf("%s (%s:%d)", m, m.URL, m.Line+1))
				b.mu.Unlock()
			}
		}
	}()
	tctx, tcancel := context.WithTimeout(ctx, 90*time.Second)
	t.Cleanup(tcancel)
	b.ctx = tctx
	return b
}

// do runs the steps; if one fails, it fails the test, with what the page
// reported and what htt gui printed.
func (b *browser) do(g *guiProcess, steps ...chromedp.Action[chromedp.Void]) {
	b.t.Helper()
	if err := chromedp.Do(b.ctx, steps...); err != nil {
		stdout, stderr := g.output()
		b.mu.Lock()
		defer b.mu.Unlock()
		b.t.Fatalf("%v\nthe page reported: %v\nhtt gui stdout: %s\nstderr: %s", err, b.problems, stdout, stderr)
	}
}

// checkConsole fails the test if a page reported an error.
func (b *browser) checkConsole() {
	b.t.Helper()
	time.Sleep(200 * time.Millisecond) // the last messages on their way
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.problems) > 0 {
		b.t.Errorf("the page reported:\n  %s", strings.Join(b.problems, "\n  "))
	}
}

// waitFor polls the page until the JavaScript condition holds.
func waitFor(what, condition string) chromedp.Action[chromedp.Void] {
	return func(ctx context.Context, tg *chromedp.Target) (chromedp.Void, error) {
		if _, err := chromedp.Poll[bool](condition, chromedp.WithPollingTimeout(30*time.Second))(ctx, tg); err != nil {
			return chromedp.Void{}, fmt.Errorf("waiting for %s: %w", what, err)
		}
		return chromedp.Void{}, nil
	}
}

// svgText is a JavaScript expression for the text the charts drew in
// the element with that id.
func svgText(id string) string {
	return fmt.Sprintf(`(document.querySelector("#%s svg") || {textContent: ""}).textContent`, id)
}

// TestGUI_InTheBrowser loads the page from the real binary in headless
// Chrome and checks it draws both views, and a known label in each,
// without a JavaScript exception or an error on the console (a failed
// load or a Content-Security-Policy violation included). It's skipped
// where there's no Chrome; HTT_TEST_CHROME names one.
func TestGUI_InTheBrowser(t *testing.T) {
	b := newBrowser(t, 1180, 1400)
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

	// The week, at the root htt gui printed: the timeline's legend and
	// labels, the totals' labels; the zoom out to the whole day.
	b.do(g,
		chromedp.Navigate(g.url+"#week/2026-11-04"),
		waitFor("the page at the root", `location.pathname === "/"`),
		waitFor("the week's timeline", svgText("timeline")+`.includes("smoke-test") && `+svgText("timeline")+`.includes("no @end")`),
		waitFor("the week's totals", svgText("summary")+`.includes("7h 0m")`),
		waitFor("the week's title", `document.querySelector("#title").textContent === "2–8 November 2026"`),
		chromedp.Click(chromedp.ID("zoom")),
		waitFor("the whole day", `document.querySelector("#zoom").textContent === "Hours logged"`),
	)
	// The month, by its route: the calendar, the charts, the table.
	b.do(g,
		chromedp.Evaluate[chromedp.Void](`location.hash = "#month/2026-11"`),
		waitFor("the month's calendar", svgText("calendar")+`.includes("7h 0m")`),
		waitFor("the month's charts", svgText("trends")+`.includes("smoke-test") && `+svgText("weeks")+`.includes("2–8 Nov") && `+svgText("weekday")+`.includes("Mon")`),
		waitFor("the month's table", `document.querySelector("#month-totals").textContent.includes("smoke-test")`),
		waitFor("the month's title", `document.querySelector("#title").textContent === "November 2026"`),
	)
	// Dark, when the system asks for it, drawn again; then the week by
	// its tab, which goes back to the week shown before.
	if _, err := chromedp.Call(b.ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "dark"}},
	}); err != nil {
		t.Fatal(err)
	}
	b.do(g,
		waitFor("the dark page", `getComputedStyle(document.body).backgroundColor === "rgb(14, 20, 23)"`),
		waitFor("the dark calendar", svgText("calendar")+`.includes("7h 0m")`),
		chromedp.Click(chromedp.ID("tab-week")),
		waitFor("the week again", `document.querySelector("#title").textContent === "2–8 November 2026" && location.hash === "#week/2026-11-04"`),
		waitFor("the dark week", svgText("timeline")+`.includes("smoke-test")`),
	)
	b.checkConsole()
}

// TestGUI_InTheBrowser_Corners draws what a review found the page
// getting wrong: categories named after Object.prototype's members; an
// entry pinned below the timeline, then another week; an average week
// above every week's bar; and an entry in the hour that repeats when
// the clocks go back.
func TestGUI_InTheBrowser_Corners(t *testing.T) {
	b := newBrowser(t, 1180, 1400)
	e := newEnv(t)
	// constructor and toString have no time: a plain object's lookup
	// found Object.prototype's functions, and the tables said NaNm.
	e.writeConfig(`check_for_updates: false
timelog:
  categories:
    - name: review
      tags: ["+review"]
    - name: constructor
      tags: ["+constructor"]
    - name: toString
      tags: ["+tostring"]
      work: false
    - name: __proto__
      tags: ["+proto"]
`)
	// July 2026, 09:00 to 17:00 Monday to Friday: 23 days, 184h, so the
	// average week, 184h ÷ 31 × 7, is 41h 32m 54s, above every week's 40h at
	// most. TZ sets htt's zone where Go reads it; Windows writes the logs
	// in its own.
	loc := time.Local
	clocksBack := runtime.GOOS != "windows"
	if clocksBack {
		var err error
		if loc, err = time.LoadLocation("Europe/London"); err != nil {
			t.Fatal(err)
		}
		e.extraEnv = append(e.extraEnv, "TZ=Europe/London")
	}
	for d := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC); d.Month() == time.July; d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			e.writeTimelog(loc, d.Format("2006-01-02"), "09:00 work +review", "17:00 @end")
		}
	}
	// London's clocks go back at 02:00 BST on 25 October 2026: from 01:50
	// BST to 01:10 GMT is 20 minutes, drawn from 01:50 to 02:10, and
	// shown with its clock times. Not on Windows, where htt's zone is the
	// system's.
	if clocksBack {
		e.writeData(filepath.Join("timelogs", "2026-10-25.log"),
			"late fix +review ts:2026-10-25T00:50:00Z\n@end ts:2026-10-25T01:10:00Z\n")
	}
	g := e.startGUI()
	// No animations: the test clicks an entry where it's drawn.
	if _, err := chromedp.Call(b.ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}},
	}); err != nil {
		t.Fatal(err)
	}

	// The week of 8 July: the categories without time have none.
	const row = `[...document.querySelectorAll("#week-totals tr")].find((tr) => tr.firstElementChild.textContent === %q).lastElementChild.textContent === "0m"`
	b.do(g,
		chromedp.Navigate(g.url+"#week/2026-07-08"),
		waitFor("the week of 8 July", `document.querySelector("#title").textContent === "6–12 July 2026"`),
		waitFor("no time in constructor, toString or __proto__", fmt.Sprintf(row, "constructor")+" && "+fmt.Sprintf(row, "toString")+" && "+fmt.Sprintf(row, "__proto__")),
		waitFor("no NaN in the week", `!document.body.textContent.includes("NaN")`),
		waitFor("a colour for __proto__", `(() => {
			const tr = [...document.querySelectorAll("#week-totals tr")].find((tr) => tr.firstElementChild.textContent === "__proto__");
			return getComputedStyle(tr.querySelector(".swatch i")).backgroundColor !== "rgba(0, 0, 0, 0)";
		})()`),
	)
	// Pin Wednesday's entry at 12:00; another week clears it.
	at, err := chromedp.Run(b.ctx, chromedp.Evaluate[[]float64](`(() => {
		const el = document.getElementById("timeline");
		const [x, y] = echarts.getInstanceByDom(el).convertToPixel({ gridIndex: 0 }, [720, 2]);
		const box = el.getBoundingClientRect();
		return [box.left + x, box.top + y];
	})()`))
	if err != nil {
		t.Fatal(err)
	}
	b.do(g,
		chromedp.MouseClickXY(at[0], at[1]),
		waitFor("the entry pinned", `document.querySelector("#detail").textContent.includes("work +review · Wed 8, 09:00–17:00")`),
		chromedp.Click(chromedp.ID("previous")),
		waitFor("the week before", `document.querySelector("#title").textContent === "29 June – 5 July 2026"`),
		waitFor("the pin gone", `document.querySelector("#detail").textContent === "Click an entry to see it here."`),
	)
	// The month: the average week drawn, dashed and labelled, though
	// it's above every bar; no NaN in its table. The page writes it as
	// htt report does: 41h 32m 54s is "41h 32m" in both, not 41h 33m.
	avgWeek := ""
	for _, line := range strings.Split(e.mustRun("report", "--month", "2026-07").stdout, "\n") {
		if f := strings.Fields(line); len(f) == 4 && f[0] == "Average" && f[1] == "week" {
			avgWeek = f[2] + " " + f[3]
		}
	}
	if avgWeek != "41h 32m" {
		t.Fatalf("htt report --month 2026-07: average week %q, want 41h 32m", avgWeek)
	}
	b.do(g,
		chromedp.Evaluate[chromedp.Void](`location.hash = "#month/2026-07"`),
		waitFor("July", `document.querySelector("#title").textContent === "July 2026"`),
		waitFor("the average week, as htt report writes it, in the weeks chart and the summary",
			svgText("weeks")+`.includes("avg `+avgWeek+`") && !!document.querySelector("#weeks svg path[stroke-dasharray]")`+
				` && document.querySelector("#month-stats").textContent.includes("`+avgWeek+`")`),
		waitFor("no NaN in the month", `!document.body.textContent.includes("NaN")`),
	)

	if clocksBack {
		b.do(g,
			chromedp.Evaluate[chromedp.Void](`location.hash = "#week/2026-10-25"`),
			waitFor("the week of 25 October", `document.querySelector("#title").textContent === "19–25 October 2026"`),
			waitFor("the entry drawn for its 20 minutes, its clock times in the tooltip", `(() => {
				const chart = echarts.getInstanceByDom(document.getElementById("timeline"));
				const option = chart && chart.getOption();
				const item = option && option.series.flatMap((s) => s.data).find((d) => d.iv && d.iv.entry === "late fix +review");
				return !!item && item.value[1] === 110 && item.value[2] === 130
					&& option.tooltip[0].formatter({ data: item }).includes("01:50–01:10 · 20m");
			})()`),
		)
	}
	b.checkConsole()
}
