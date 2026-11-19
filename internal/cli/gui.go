package cli

import (
	gocontext "context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bricef/htt/internal/gui"
	"github.com/cli/browser"
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
		ctx, stop := signal.NotifyContext(gocontext.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return server.ListenAndServe(ctx, guiPort, func(url string) {
			fmt.Printf("htt gui: %s (Ctrl-C to stop)\n", url)
			if configErr != nil {
				fmt.Fprintf(os.Stderr, "warning: %v; the page shows it until you fix it and restart htt gui\n", configErr)
			}
			if !guiNoOpen {
				go openBrowser(url)
			}
		})
	},
}

// openBrowser opens url in the default browser, or says on stderr that
// it couldn't. The opener's own output (xdg-open is chatty) is dropped.
func openBrowser(url string) {
	browser.Stdout, browser.Stderr = io.Discard, io.Discard
	if err := browser.OpenURL(url); err != nil {
		var exit interface{ ExitCode() int }
		if errors.As(err, &exit) {
			err = fmt.Errorf("the opener exited %d", exit.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "htt gui: couldn't open a browser (%v); open the address above yourself\n", err)
	}
}

func init() {
	GUI.Flags().IntVar(&guiPort, "port", 0, "the port to serve on, on 127.0.0.1; 0 picks a free one")
	GUI.Flags().BoolVar(&guiNoOpen, "no-open", false, "print the address without opening a browser")
	RootCmd.AddCommand(GUI)
}
