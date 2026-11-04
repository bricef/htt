// Package update checks GitHub for a newer release of htt.
//
// A check makes at most one request a day, to one URL, and treats every
// failure as "no news": it never fails a command and prints nothing
// itself. Callers decide whether a check may run at all and how to show
// what it found.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"golang.org/x/mod/semver"
)

// releasesURL is the only URL a check contacts. It's a variable so
// tests can point it at a local server; no config or environment
// variable changes it at run time.
var releasesURL = "https://api.github.com/repos/bricef/htt/releases/latest"

// requestTimeout bounds the request; a variable so tests can shorten it.
var requestTimeout = 3 * time.Second

const (
	// Interval is the shortest time between two checks.
	Interval = 24 * time.Hour

	installCommand = "go install github.com/bricef/htt/cmd/htt@latest"
)

// CurrentVersion is the running binary's version from its build info:
// vX.Y.Z for a release install, a pseudo-version for builds between
// releases, and "" when Go recorded none.
func CurrentVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi.Main.Version == "(devel)" {
		return ""
	}
	return bi.Main.Version
}

// DefaultStatePath is where checks remember the last attempt: the
// user's cache directory (~/.cache/htt on Linux), or "" if there is
// none, in which case no check runs.
func DefaultStatePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "htt", "update-check.json")
}

// state is what a check remembers between runs.
type state struct {
	CheckedAt time.Time `json:"checked_at"` // the last attempt, whatever its outcome
	Latest    string    `json:"latest"`     // the latest release's tag; "" if none
	URL       string    `json:"url"`        // its release page
}

func loadState(path string) state {
	var s state
	b, err := os.ReadFile(path)
	if err != nil {
		return state{}
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return state{}
	}
	return s
}

func saveState(path string, s state) error {
	if path == "" {
		return errors.New("no cache directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Config is what Start needs. Now and Client default to the real clock
// and an HTTP client with requestTimeout.
type Config struct {
	Current   string // the running binary's version
	StatePath string // where to remember the last check
	Now       func() time.Time
	Client    *http.Client
}

// Release is a published release newer than the running binary.
type Release struct {
	Version string // the release's tag
	Current string // the running binary's version
	URL     string // the release page
}

// Notice is the message for the end of a CLI command.
func (r Release) Notice() string {
	return fmt.Sprintf("A new version of htt is available: %s (you have %s).\n"+
		"  Update:        %s\n"+
		"  Release notes: %s\n"+
		"Turn this check off with check_for_updates: false in your config file.",
		r.Version, r.Current, installCommand, r.URL)
}

// Short is the one-line form, for the TUI's status line.
func (r Release) Short() string {
	return fmt.Sprintf("htt %s is out (you have %s): %s", r.Version, r.Current, installCommand)
}

// Check is one run's update check. Start creates it; once Done is
// closed, Newer and Err report how it went.
type Check struct {
	current string
	done    chan struct{}
	asked   bool  // this run asked GitHub (and got an answer)
	found   state // what GitHub said
	err     error // why the check didn't get an answer
}

// Start begins a check if one is due: the running version is known, and
// the last attempt was a day or more ago. It records the attempt first,
// so a failed or abandoned request still counts, then asks GitHub in
// the background. Start never blocks on the network and never fails;
// problems surface only through Err.
func Start(cfg Config) *Check {
	c := &Check{current: cfg.Current, done: make(chan struct{})}
	if !semver.IsValid(cfg.Current) {
		close(c.done)
		return c
	}
	now := time.Now
	if cfg.Now != nil {
		now = cfg.Now
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}

	prev := loadState(cfg.StatePath)
	since := now().Sub(prev.CheckedAt)
	if !prev.CheckedAt.IsZero() && since >= 0 && since < Interval {
		close(c.done)
		return c
	}
	attempt := prev
	attempt.CheckedAt = now()
	if err := saveState(cfg.StatePath, attempt); err != nil {
		// Without a record of this attempt every run would ask again, so
		// don't ask at all.
		c.err = fmt.Errorf("record update check: %w", err)
		close(c.done)
		return c
	}

	go func() {
		defer close(c.done)
		tag, url, err := latestRelease(client, cfg.Current)
		if err != nil {
			c.err = err
			return
		}
		attempt.Latest, attempt.URL = tag, url
		c.asked, c.found = true, attempt
		if err := saveState(cfg.StatePath, attempt); err != nil {
			c.err = fmt.Errorf("record update check: %w", err)
		}
	}()
	return c
}

// Done is closed once the check has finished: at once when none was
// due, otherwise when the request completes or fails.
func (c *Check) Done() <-chan struct{} {
	return c.done
}

// Wait waits up to timeout for the check to finish and reports whether
// it has.
func (c *Check) Wait(timeout time.Duration) bool {
	select {
	case <-c.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Newer reports the release this run found, if it's newer than the
// running binary. It's false until Done, and on any run that didn't ask
// GitHub, so a notice appears at most once per check.
func (c *Check) Newer() (Release, bool) {
	select {
	case <-c.done:
	default:
		return Release{}, false
	}
	if !c.asked || c.found.Latest == "" || semver.Compare(c.found.Latest, c.current) <= 0 {
		return Release{}, false
	}
	return Release{Version: c.found.Latest, Current: c.current, URL: c.found.URL}, true
}

// Err says why the check got no answer (offline, an HTTP error, an
// unwritable cache, …). It's nil until Done and after a successful
// check, including one that found no release.
func (c *Check) Err() error {
	select {
	case <-c.done:
		return c.err
	default:
		return nil
	}
}

// latestRelease asks GitHub for the latest published release (drafts
// and pre-releases excluded). A repository with no release yet answers
// 404, which is an empty tag rather than an error.
func latestRelease(client *http.Client, current string) (tag, url string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "htt/"+current)

	// Follow redirects only within the same host: GitHub redirects
	// renamed repositories, but nothing should take the check elsewhere.
	c := *client
	c.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if r.URL.Host != via[0].URL.Host {
			return fmt.Errorf("refusing redirect to %s", r.URL.Host)
		}
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return nil
	}

	resp, err := c.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("ask GitHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", "", nil
	default:
		return "", "", fmt.Errorf("ask GitHub: %s", resp.Status)
	}

	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", "", fmt.Errorf("read GitHub's answer: %w", err)
	}
	if !semver.IsValid(body.TagName) {
		return "", "", fmt.Errorf("latest release tag %q isn't a version", body.TagName)
	}
	return body.TagName, body.HTMLURL, nil
}
