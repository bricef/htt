// Package gui serves htt's GUI: a local web page that draws the time
// log, and the JSON it draws from. `htt gui` runs it.
//
// The server is local and private. It listens on 127.0.0.1 only; every
// path starts with a random token, new each run, that the address htt
// prints and opens carries, and anything else is a 404; a request whose
// Host isn't 127.0.0.1:<port> or localhost:<port> is refused, so a page
// elsewhere can't reach it through DNS rebinding; and it only reads,
// answering GET and HEAD and nothing else.
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
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
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

// Server serves the GUI under a token.
type Server struct {
	cfg   Config
	token string
}

// New returns a server for cfg under a new random token of 128 bits.
func New(cfg Config) (*Server, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("make a token: %w", err)
	}
	return newServer(cfg, hex.EncodeToString(b)), nil
}

func newServer(cfg Config, token string) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Server{cfg: cfg, token: token}
}

// Path is the path the GUI's page is at: its token, between slashes.
func (s *Server) Path() string {
	return "/" + s.token + "/"
}

// ListenAndServe serves the GUI on 127.0.0.1:port, port 0 picking a free
// one, until ctx is done; then it stops taking requests, lets those
// under way finish (for up to five seconds), and returns nil. Once it
// listens, it calls ready with the page's address,
// http://127.0.0.1:<port>/<token>/.
func (s *Server) ListenAndServe(ctx context.Context, port int, ready func(url string)) error {
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
		ready("http://" + l.Addr().String() + s.Path()) // the address it listens on
	}
	select {
	case err := <-served: // it stopped by itself
		return err
	case <-ctx.Done():
	}
	stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(stop); err != nil {
		return fmt.Errorf("stop the server: %w", err)
	}
	if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler serves the GUI to requests addressed to 127.0.0.1:port or
// localhost:port, under the server's token: the page and its assets,
// and the API under api/.
func (s *Server) Handler(port int) http.Handler {
	files := assets()
	route := func(w http.ResponseWriter, r *http.Request) {
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
	}

	hosts := []string{"127.0.0.1:" + strconv.Itoa(port), "localhost:" + strconv.Itoa(port)}
	prefix := s.Path()
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
		path := r.URL.Path
		if subtle.ConstantTimeCompare([]byte(path), []byte(strings.TrimSuffix(prefix, "/"))) == 1 {
			http.Redirect(w, r, prefix, http.StatusFound)
			return
		}
		if len(path) < len(prefix) || subtle.ConstantTimeCompare([]byte(path[:len(prefix)]), []byte(prefix)) != 1 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "htt gui only reads: GET and HEAD", http.StatusMethodNotAllowed)
			return
		}
		// The rest of the path, from its slash, as it is: the API's paths
		// are matched exactly, and a file's must be a valid fs path, so
		// "..", "." and empty elements reach nothing.
		r2 := r.Clone(r.Context())
		r2.URL.Path = path[len(prefix)-1:]
		r2.URL.RawPath = ""
		route(w, r2)
	})
}

// contentSecurityPolicy keeps the page to its own assets: no script,
// style, font or connection from elsewhere. Styles may be inline, as
// the charts' tooltips set them.
const contentSecurityPolicy = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// slicesContainsFold reports whether host is one of hosts, ignoring
// case, as host names are.
func slicesContainsFold(hosts []string, host string) bool {
	for _, h := range hosts {
		if strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}
