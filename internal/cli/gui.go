package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bricef/htt/internal/gui"
	"github.com/spf13/cobra"
)

var (
	guiPort   int
	guiNoOpen bool
)

var GUI = &cobra.Command{
	Use:   "gui",
	Short: "See the time log in your browser.",
	Long: `See the time log in your browser: htt serves a page from your time
logs, on this computer only, prints its address and opens it in your
default browser. It serves until you stop it with Ctrl-C.

The page has two views, each with previous, next and this week or month:

  Week   a timeline of each day, Monday to Sunday, every entry coloured
         by its category; and the week's totals per day and per category
  Month  worked time per day on a calendar; each week's totals and the
         average week; the average weekday; each category's hours per
         week over the last 7 days, day by day; and a table of the
         categories, the 1st to the 15th against the 16th to the end

Categories come from timelog.categories in the config, as for htt report
(see htt help report); a config htt can't use shows its error in the
page. The config is read when htt gui starts: restart it after editing.
The page refreshes itself every minute while it shows today.

The address carries a random token, new each time, and htt answers
nothing without it; it listens on 127.0.0.1 only, and only reads.

Examples:
  htt gui                   # serve on a free port and open the browser
  htt gui --no-open         # just print the address
  htt gui --port 8080       # serve on port 8080`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if guiPort < 0 || guiPort > 65535 {
			return fmt.Errorf("--port: want 0 to 65535 (0 picks a free port), got %d", guiPort)
		}
		categories, configErr := timelogCategoriser() // nil on an error: the page shows the error
		server, err := gui.New(gui.Config{
			Timelogs:   baseTimelogRepo(), // dates htt can't read: no warnings on every refresh
			Categories: categories,
			ConfigErr:  configErr,
			Now:        time.Now,
		})
		if err != nil {
			return err
		}
		// Ctrl-C (or SIGTERM) stops it; a second doesn't wait for the
		// requests under way.
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		redirect := "" // the directory of the file the browser was opened on, removed when htt gui stops
		defer func() {
			if redirect != "" {
				_ = os.RemoveAll(redirect)
			}
		}()
		return server.ListenAndServe(signals, guiPort, func(url string) {
			fmt.Printf("htt gui: %s (Ctrl-C to stop)\n", url)
			if configErr != nil {
				fmt.Fprintf(os.Stderr, "warning: %v; the page shows it until you fix it and restart htt gui\n", configErr)
			}
			if !guiNoOpen {
				redirect = openBrowser(url)
			}
		})
	},
}

// openBrowser opens url in the default browser, or says on stderr that
// it couldn't, and returns the directory to remove when htt gui stops
// ("" for none).
//
// The opener (xdg-open, open, …) is given a file, not url: a program's
// arguments are there for any user of the computer to read (ps), and
// url's token is the key to the time log. The file, in a new directory
// that only you can read, sends the browser on to url, as Jupyter's
// does.
func openBrowser(url string) (dir string) {
	dir, file, err := writeRedirect(url)
	if err != nil {
		cantOpen(err)
		return ""
	}
	go func() {
		if err := openFile(file); err != nil {
			cantOpen(err)
		}
	}()
	return dir
}

func cantOpen(err error) {
	fmt.Fprintf(os.Stderr, "htt gui: couldn't open a browser (%v); open the address above yourself\n", err)
}

// writeRedirect writes redirectPage(url) to open.html in a new
// directory in the system's temporary one, the directory readable by
// you alone (0700) and the file too (0600).
func writeRedirect(url string) (dir, file string, err error) {
	dir, err = os.MkdirTemp("", "htt-gui-")
	if err != nil {
		return "", "", err
	}
	file = filepath.Join(dir, "open.html")
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, err = f.WriteString(redirectPage(url))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", "", err
	}
	return dir, file, nil
}

// redirectPage is a page that sends the browser on to url at once, by a
// meta refresh and by script (which leaves the page out of the
// history), with a link should neither work. It's a file of the user's,
// not one htt serves: no Content-Security-Policy keeps out its script.
func redirectPage(url string) string {
	attr := html.EscapeString(url)
	js, _ := json.Marshal(url) // a JavaScript string; <, > and & escaped
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="0; url=` + attr + `">
<title>htt gui</title>
<script>location.replace(` + string(js) + `)</script>
</head>
<body>
<p>Opening htt gui: <a href="` + attr + `">` + attr + `</a></p>
</body>
</html>
`
}

func init() {
	GUI.Flags().IntVar(&guiPort, "port", 0, "the port to serve on, on 127.0.0.1; 0 picks a free one")
	GUI.Flags().BoolVar(&guiNoOpen, "no-open", false, "print the address without opening a browser")
	RootCmd.AddCommand(GUI)
}
