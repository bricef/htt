# TUI themes: configurable colours and named palettes

**Date:** 2026-11-03
**Status:** Complete (2026-11-03)

## Outcome

Shipped as planned. Notes from implementing:

- **lipgloss rounds colours**: `#f92672` is emitted as blue 113, not
  114. The view tests accept each channel within ±1.
- **Checked on a real terminal.** With no theme configured, the new
  build draws exactly the same eight colours as the build before
  themes; with `monokai` and an `accent` override it draws Monokai's
  palette with the override. A preview of all nine themes rendered
  from the real TUI looked right; Solarized's selection highlight is
  faint (about 1.15:1 against its background), which is the palette's
  own base02/base2 and is left as is.
- **One gap**: `cli` passing the resolved theme into `tui.Model` is
  covered only by that real-terminal check, since the TUI needs a
  terminal to start. Every other piece has a test, and each test was
  shown to fail when its piece is broken.

## Motivation

The TUI's colours are hard-coded: seven package-level variables in
`internal/tui/interactive.go`, baked into package-level styles. Users
can't adjust them, and can't pick a familiar palette (Monokai, VS
Code, …) to match their terminal.

## Decisions captured

From the design dialogue (2026-11-03):

1. **Seven colour roles, one per colour in use today**, named for what
   they paint:

   | Role         | Paints                                                                        | Today's value |
   | ------------ | ----------------------------------------------------------------------------- | ------------- |
   | `foreground` | the title; text on the selected tab and task; the fill of the done tab and the new-task input | `#cfe0c3` |
   | `background` | text drawn on that fill                                                        | `#192b31` |
   | `selection`  | the selected tab and task                                                      | `#40798c` |
   | `accent`     | the new-task prompt and the status line                                        | `#70a9a1` |
   | `muted`      | borders, keys in the help bar, the input placeholder                           | `#626262` |
   | `help`       | descriptions in the help bar                                                   | `#4A4A4A` |
   | `separator`  | separators in the help bar                                                     | `#3C3C3C` |

2. **The `default` theme is today's palette**, so an unconfigured TUI
   looks exactly as before.

3. **Built-in themes** from each palette's canonical colours:
   `default`, `monokai`, `vscode` (Dark+), `vscode-light` (Light+),
   `dracula`, `nord`, `solarized-dark`, `solarized-light`, `gruvbox`.

4. **Config lives under a `tui:` section:**

   ```yaml
   tui:
     theme: monokai
     colors:
       accent: "#f92672"
   ```

   The default theme comes first, then the named theme, then any
   `colors` overrides. A colour is a hex value (`#rgb` / `#rrggbb`) or
   an ANSI 256-colour number (`0`–`255`). Hex values must be quoted:
   in YAML an unquoted `#` starts a comment, leaving the value empty,
   and the error says so.

5. **Bad config fails before the TUI starts**, like the log flags: an
   unknown theme, an unknown role or an invalid colour is an error
   naming the valid choices. Theme names are case-insensitive.

6. **The TUI still doesn't paint the screen background.** Rows and
   tabs that aren't selected use the terminal's own colours, as today,
   so a theme recolours htt's elements rather than the whole screen;
   pick one that suits the terminal's background. Painting the full
   background is a larger change (every style, and the help and
   text-input components, would have to carry it) and is left out.

7. **The tui package doesn't read config.** `cli` reads the `tui`
   section and calls `tui.ResolveTheme(name, overrides)`; `tui.Model`
   takes an `Options` struct (logger, theme) so the constructor stops
   growing a parameter per feature. Zero options mean a discard logger
   and the default theme.

## Shape

- `internal/tui/theme.go`: `Theme`, the role list, the built-in
  themes, `ResolveTheme`, `ThemeNames`, and `styles` built from a
  theme.
- `internal/tui/interactive.go`, `TaskListView.go`: drop the colour
  globals; `model.styles` drives `View` and the task list.
- `internal/cli/interact.go`: read `tui.theme` / `tui.colors`,
  resolve before anything else, list the themes in `--help`.
- `internal/vars`: the two config keys and a map accessor;
  `cmd/htt/main.go` defaults `tui.theme` to `default`, so a fresh
  config shows the setting.

## Tests

- `ResolveTheme`: default when unset, named themes, overrides, case,
  ANSI numbers, and each error (unknown theme, unknown role, bad or
  empty colour).
- The default theme pins today's seven values.
- Every built-in theme defines all roles with valid colours and keeps
  text readable: WCAG contrast of at least 3:1 for `foreground` on
  `selection`, `background` on `foreground`, and `accent` on
  `background`.
- `View` uses the model's theme: rendered with a true-colour profile,
  the selected row carries the theme's `selection` colour.
- e2e: an unknown theme and a bad colour in the config fail
  `htt interactive` before the TUI starts.
- Real terminal: the default theme's colours unchanged, and a named
  theme plus an override applied.

## Out of scope

- Painting the screen background (see decision 6).
- A `--theme` flag or a theme preview command.
- Light/dark adaptive colours.
- Theming the CLI's own colours (`htt todo show` priorities), which
  use a separate library.
