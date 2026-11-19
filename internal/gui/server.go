// Package gui serves htt's GUI: a local web page that draws the time
// log, and the JSON it draws from. `htt gui` runs it.
//
// The server is local. It listens on 127.0.0.1 only, so no other
// computer reaches it, though any account on this one can while it
// runs: the page is at http://127.0.0.1:<port>/, with no token, as Go's
// own `go tool pprof -http` serves its UI. A request whose Host isn't
// 127.0.0.1:<port> or localhost:<port> (or, on port 80, either without
// the port) is refused, so a page elsewhere can't reach it through DNS
// rebinding; it sends no CORS headers, so a page elsewhere can't read
// what it answers; it answers the GUI's own page only, refusing a
// request with another Origin or one another site's page makes (by its
// Sec-Fetch-Site), but for a link followed to open the page; and it
// only reads, answering GET and HEAD and nothing else.
//
// Go computes, the page draws: the API's handlers read each day's
// timesheet and add them up with the domain's period aggregates (see
// view.go), and the page, embedded in the binary (assets.go), only lays
// the figures out. The CLI hands the package what it reads: a timelog
// repository, the categories (or the config's error) and a clock. It
// imports neither the CLI nor storage.
package gui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// Config is what the GUI shows and how: the CLI builds it.
type Config struct {
	// Timelogs is where each day's time log is read from.
	Timelogs domain.TimelogRepository
	// Categories sort entries into categories; nil has none, and
	// everything is uncategorised.
	Categories *domain.Categoriser
	// ConfigErr is the config's error when htt can't use its timelog
	// section: the API answers with it, naming the key, rather than
	// with time in the wrong categories. The config is read once, when
	// htt gui starts.
	ConfigErr error
	// Now is the clock, and its location the local zone: days run from
	// midnight to midnight there.
	Now func() time.Time
}

// Server serves the GUI.
type Server struct {
	cfg   Config
	grace time.Duration // how long requests under way have to finish once it's stopped
}

// shutdownGrace is how long requests under way have to finish once the
// server is stopped: a moment, as the API answers in milliseconds.
const shutdownGrace = time.Second

// New returns a server for cfg.
func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Server{cfg: cfg, grace: shutdownGrace}
}

// ListenAndServe serves the GUI on 127.0.0.1:port, port 0 picking a free
// one, until a signal on stop asks it to stop (the CLI's Ctrl-C). Then it
// stops taking requests, gives those under way a moment to finish (a
// second, cut short by a second signal), closes the connections left (a
// browser's idle ones among them), and returns nil. Once it listens, it
// calls ready with the page's address, http://127.0.0.1:<port>/.
func (s *Server) ListenAndServe(stop <-chan os.Signal, port int, ready func(url string)) error {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("listen on 127.0.0.1:%d: %w", port, err)
	}
	port = l.Addr().(*net.TCPAddr).Port
	srv := &http.Server{
		Handler:           s.Handler(port),
		ReadHeaderTimeout: 10 * time.Second,
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(l) }()
	if ready != nil {
		ready("http://" + l.Addr().String() + "/") // the address it listens on
	}
	select {
	case err := <-served: // it stopped by itself
		return err
	case <-stop:
	}
	grace, cancel := context.WithTimeout(context.Background(), s.grace)
	defer cancel()
	go func() {
		select {
		case <-stop: // asked again: at once
			cancel()
		case <-grace.Done():
		}
	}()
	if err := srv.Shutdown(grace); err != nil {
		_ = srv.Close() // the moment is over: close what's left
	}
	if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler serves the GUI to requests addressed to 127.0.0.1:port or
// localhost:port (without the port, too, when it's 80, as browsers
// leave the default port out), from the GUI's own page or a person
// opening it (see foreign): the page at /, its assets at their paths,
// and the API under /api/.
func (s *Server) Handler(port int) http.Handler {
	files := assets()
	hosts := []string{"127.0.0.1:" + strconv.Itoa(port), "localhost:" + strconv.Itoa(port)}
	if port == 80 { // the default, which a browser leaves out of Host
		hosts = append(hosts, "127.0.0.1", "localhost")
	}
	origins := make([]string, len(hosts)) // the GUI's own, as its page's requests carry them
	for i, host := range hosts {
		origins[i] = "http://" + host
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", contentSecurityPolicy)

		if !slicesContainsFold(hosts, r.Host) {
			http.Error(w, "htt gui answers to "+hosts[0]+" and "+hosts[1]+" only", http.StatusForbidden)
			return
		}
		if why := foreign(r, origins); why != "" {
			http.Error(w, "htt gui answers its own page only: "+why, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "htt gui only reads: GET and HEAD", http.StatusMethodNotAllowed)
			return
		}
		// The path as it is: the API's paths are matched exactly, and a
		// file's must be a valid fs path, so "..", "." and empty elements
		// reach nothing.
		switch p := r.URL.Path; {
		case p == "/api/week":
			s.handleWeek(w, r)
		case p == "/api/month":
			s.handleMonth(w, r)
		case p == "/api" || strings.HasPrefix(p, "/api/"):
			writeError(w, r, http.StatusNotFound, errors.New("no such API: want api/week or api/month"))
		default:
			files.ServeHTTP(w, r)
		}
	})
}

// foreign says why r is refused as another site's doing, or returns ""
// when it's the GUI's own page asking, or a person opening it.
//
// An Origin, when r has one, must be the GUI's own, one of origins;
// "null", from a sandboxed frame or a file, isn't. And by the fetch
// metadata browsers send, a request another site's page makes
// (Sec-Fetch-Site same-site, as a page on another port of localhost
// is, or cross-site) is refused, fetches, images, scripts, styles and
// frames alike, unless it's a link followed to open the page: a GET
// navigating to a document at the top level. "same-origin" is the
// page's own requests; "none" an address typed, a bookmark or htt's
// opener. Without Sec-Fetch-Site (curl, an old browser) the Host and
// Origin checks are all there is.
//
// It doesn't use http.CrossOriginProtection, which leaves GET and HEAD
// alone, the only methods the server answers; should the GUI ever
// write, that's the guard for its writes.
func foreign(r *http.Request, origins []string) string {
	if o, ok := r.Header["Origin"]; ok && (len(o) != 1 || !slicesContainsFold(origins, o[0])) {
		return "the request comes from another origin"
	}
	sites := r.Header.Values("Sec-Fetch-Site")
	if len(sites) == 0 || len(sites) == 1 && (sites[0] == "same-origin" || sites[0] == "none") {
		return ""
	}
	if r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Mode") == "navigate" && r.Header.Get("Sec-Fetch-Dest") == "document" {
		return "" // a link followed to open the page
	}
	return "another site's page asked for it"
}

// contentSecurityPolicy keeps the page to its own assets: no script,
// style, font or connection from elsewhere. Styles may be inline, as
// the charts' tooltips set them.
const contentSecurityPolicy = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// slicesContainsFold reports whether host (or an origin) is one of
// hosts, ignoring case, as host names are.
func slicesContainsFold(hosts []string, host string) bool {
	for _, h := range hosts {
		if strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}
