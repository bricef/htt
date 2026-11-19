package cli

import (
	"fmt"
	"os"
	"os/signal"
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

It listens on 127.0.0.1 only, and only reads. The address is plain,
http://127.0.0.1:<port>/, the same each time with --port, so you can
bookmark it; while htt gui runs, anyone with an account on this
computer can open it too.

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
		server := gui.New(gui.Config{
			Timelogs:   baseTimelogRepo(), // dates htt can't read: no warnings on every refresh
			Categories: categories,
			ConfigErr:  configErr,
			Now:        time.Now,
		})
		// Ctrl-C (or SIGTERM) stops it; a second doesn't wait for the
		// requests under way.
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		return server.ListenAndServe(signals, guiPort, func(url string) {
			fmt.Printf("htt gui: %s (Ctrl-C to stop)\n", url)
			if configErr != nil {
				fmt.Fprintf(os.Stderr, "warning: %v; the page shows it until you fix it and restart htt gui\n", configErr)
			}
			if !guiNoOpen {
				openBrowser(url)
			}
		})
	},
}

// openBrowser opens url in the default browser, in the background, or
// says on stderr that it couldn't.
func openBrowser(url string) {
	go func() {
		if err := openURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "htt gui: couldn't open a browser (%v); open the address above yourself\n", err)
		}
	}()
}

func init() {
	GUI.Flags().IntVar(&guiPort, "port", 0, "the port to serve on, on 127.0.0.1; 0 picks a free one")
	GUI.Flags().BoolVar(&guiNoOpen, "no-open", false, "print the address without opening a browser")
	RootCmd.AddCommand(GUI)
}
