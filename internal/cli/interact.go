package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bricef/htt/internal/tui"
	"github.com/bricef/htt/internal/vars"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	tuiLogFile  string
	tuiLogLevel = "debug"
)

var Interactive = &cobra.Command{
	Use:     "interactive",
	Short:   "Enter interactive mode",
	Aliases: []string{"i"},
	Long: fmt.Sprintf(`Enter interactive mode: a full-screen view over the same data files
as the CLI.

Colours come from the tui section of the config file: pick a built-in
theme, then override any of its colours.

  tui:
    theme: monokai
    colors:
      accent: "#f92672"

Themes: %s.
Colours: %s. Each takes a hex value (#rrggbb or #rgb; quote it, or
YAML reads the # as a comment) or an ANSI colour number (0-255). The
TUI doesn't paint the screen background, so pick a theme that suits
your terminal.

The TUI writes nothing outside its own screen. To see what it is doing,
pass --log-file: diagnostics are appended to that file at debug level,
the most detailed, unless --log-level picks another:

  debug   every action the TUI runs (each keypress)
  info    the TUI starting and exiting
  warn    an action that can't run right now (e.g. editing with no $EDITOR)
  error   an action or the editor failed; most action failures also
          close the TUI, editor failures don't

Each level also includes the levels below it in this list.

Examples:
  htt interactive --log-file ~/htt-tui.log
  htt interactive --log-file ~/htt-tui.log --log-level warn`,
		strings.Join(tui.ThemeNames(), ", "), strings.Join(tui.RoleNames(), ", ")),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("log-level") && tuiLogFile == "" {
			return errors.New("--log-level needs --log-file")
		}
		theme, err := tui.ResolveTheme(vars.Get(vars.ConfigKeyTUITheme), vars.GetStringMapString(vars.ConfigKeyTUIColors))
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}
		logger, closeLog, err := tuiLogger(tuiLogFile, tuiLogLevel)
		if err != nil {
			return err
		}
		defer func() { _ = closeLog() }()

		r := repo()
		ctx, err := r.CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		p := tea.NewProgram(
			tui.Model(r, ctx, tui.Options{Logger: logger, Theme: theme}),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		logger.Info("tui started", "context", ctx.Name)
		if _, err := p.Run(); err != nil {
			logger.Error("tui failed", "err", err)
			return fmt.Errorf("tui exited: %w", err)
		}
		logger.Info("tui exited")
		return nil
	},
}

// logLevels maps --log-level values onto slog levels.
var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// tuiLogger builds the TUI's logger from --log-file and --log-level.
// With no file it discards everything. The level is checked before
// the file is opened, so a typo doesn't leave an empty log behind.
// The returned close func is always safe to call.
func tuiLogger(path, level string) (*slog.Logger, func() error, error) {
	lvl, ok := logLevels[strings.ToLower(level)]
	if !ok {
		return nil, nil, fmt.Errorf("invalid --log-level %q (want debug, info, warn or error)", level)
	}
	if path == "" {
		return slog.New(slog.DiscardHandler), func() error { return nil }, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open --log-file: %w", err)
	}
	return slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: lvl})), f.Close, nil
}

func init() {
	Interactive.Flags().StringVar(&tuiLogFile, "log-file", "",
		"Append TUI diagnostics to this file (off by default; logs at debug level unless --log-level is set)")
	Interactive.Flags().StringVar(&tuiLogLevel, "log-level", "debug",
		"Lowest level written to --log-file: debug, info, warn or error")
	RootCmd.AddCommand(Interactive)
}
