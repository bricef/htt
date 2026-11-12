package tui

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/bricef/htt/internal/dates"
	"github.com/bricef/htt/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// inputMode is what the task input is open for.
type inputMode int

const (
	inputClosed inputMode = iota
	inputAdd              // a new task for the current context
	inputEdit             // the task at model.editIndex
)

// draft is the task input read as the line it would save.
type draft struct {
	raw      string   // the line that would be saved; "" when the input is empty
	problems []string // why it can't be saved yet
	warnings []string // it can be saved, but may not mean what was meant
}

// readDraft reads what's typed: due phrases resolved to dates and, for
// a new task, the created-on date AddTask will stamp (now's day), so raw
// is exactly the line a save writes.
func readDraft(value string, now time.Time, adding bool) draft {
	value = strings.TrimSpace(value)
	if value == "" {
		return draft{}
	}
	raw, problems := resolveDue(value, now)
	d := draft{raw: raw, problems: problems}
	if len(problems) > 0 {
		return d
	}
	task, err := domain.NewTask(raw)
	if err != nil {
		d.problems = append(d.problems, err.Error())
		return d
	}
	if adding {
		d.raw = task.StampCreated(now).Raw
	}
	d.warnings = wordWarnings(d.raw, adding)
	return d
}

// dueWord finds due: annotations as typed: due:friday, or a quoted
// phrase, due:"in two weeks" (the closing quote may be missing while
// it's being typed).
var dueWord = regexp.MustCompile(`(^|\s)due:("[^"]*"?|\S*)`)

var dateShaped = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// resolveDue rewrites each due: value that isn't already a date as the
// date it names (due:friday becomes due:2026-05-22), using the same
// phrases as `htt todo add --due`. A value it can't read is a problem:
// saving it would store a due date nothing can use.
func resolveDue(value string, now time.Time) (string, []string) {
	var problems []string
	out := dueWord.ReplaceAllStringFunc(value, func(match string) string {
		lead := match[:len(match)-len(strings.TrimLeft(match, " \t"))]
		given := strings.TrimPrefix(match[len(lead):], "due:")
		var phrase string
		switch {
		case given == "":
			problems = append(problems, "due: needs a date or a phrase, like due:friday")
			return match
		case strings.HasPrefix(given, `"`):
			if len(given) < 2 || !strings.HasSuffix(given, `"`) {
				problems = append(problems, fmt.Sprintf("due:%s needs a closing quote", given))
				return match
			}
			phrase = given[1 : len(given)-1]
		case dateShaped.MatchString(given):
			if _, ok := domain.ParseLocalDate(given); !ok {
				problems = append(problems, fmt.Sprintf("due:%s isn't a real date", given))
			}
			return match
		default:
			// due:next-week reads as "next week".
			phrase = strings.NewReplacer("-", " ", "_", " ").Replace(given)
		}
		day, err := dates.ParsePhrase(phrase, now)
		if err != nil {
			problems = append(problems, fmt.Sprintf("can't tell what date due:%s means", given))
			return match
		}
		return lead + "due:" + day.Format("2006-01-02")
	})
	return out, problems
}

var priorityShaped = regexp.MustCompile(`^\([A-Za-z]\)$`)

// wordWarnings points out words that will be saved, but not as what
// they look like.
func wordWarnings(raw string, adding bool) []string {
	var out []string
	for i, tk := range domain.Tokens(raw) {
		word := raw[tk.Start:tk.End]
		switch {
		case tk.Kind == domain.TokenDone && adding:
			out = append(out, `"x" at the start marks the task done`)
		case tk.Kind == domain.TokenPriority && !slices.Contains([]string{"(A)", "(B)", "(C)"}, word):
			out = append(out, fmt.Sprintf("%s: htt's priorities are (A), (B) and (C)", word))
		case tk.Kind == domain.TokenWord && priorityShaped.MatchString(word):
			if word != strings.ToUpper(word) {
				out = append(out, fmt.Sprintf("%s is text: priorities are capitals, %s", word, strings.ToUpper(word)))
			} else if i > 0 {
				out = append(out, fmt.Sprintf("%s is text: a priority goes at the start", word))
			}
		case tk.Kind == domain.TokenWord && dateShaped.MatchString(word):
			if _, ok := domain.ParseLocalDate(word); !ok {
				out = append(out, fmt.Sprintf("%s isn't a real date, so it's text", word))
			}
		}
	}
	return out
}

// knownTags gathers the tags used across every context but the
// archive, for completion: per sigil, most used first, then
// alphabetically.
func knownTags(repo domain.Repository) map[string][]string {
	counts := map[string]map[string]int{}
	names, err := repo.ContextNames()
	if err != nil {
		return nil
	}
	for _, name := range names {
		if name == domain.ArchiveContextName {
			continue
		}
		ctx, err := repo.Context(name)
		if err != nil {
			continue
		}
		for _, task := range ctx.Tasks {
			for sigil, tags := range task.Tags {
				if counts[sigil] == nil {
					counts[sigil] = map[string]int{}
				}
				for _, tag := range tags {
					counts[sigil][tag]++
				}
			}
		}
	}
	out := map[string][]string{}
	for sigil, c := range counts {
		out[sigil] = slices.SortedFunc(maps.Keys(c), func(a, b string) int {
			return cmp.Or(c[b]-c[a], strings.Compare(a, b))
		})
	}
	return out
}

// tagSuggestions are the completions for the input's last word when it
// starts a tag: the whole value with that word replaced by each known
// tag of its kind. (The text input completes whole values by prefix.)
func tagSuggestions(value string, tags map[string][]string) []string {
	start := strings.LastIndexAny(value, " \t") + 1
	word := value[start:]
	if word == "" || !strings.ContainsAny(word[:1], "+@#") {
		return nil
	}
	var out []string
	for _, tag := range tags[word[:1]] {
		if tag != word {
			out = append(out, value[:start]+tag)
		}
	}
	return out
}

// inputKeys are the keys the task input answers to, for the help bar.
type inputKeys struct{}

func (inputKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "complete tag")),
		key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n/p", "other tags")),
	}
}

func (k inputKeys) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }

// openInput opens the task input: empty for a new task, or holding the
// task being edited.
func (m *model) openInput(mode inputMode, value string, editIndex int) {
	m.input = mode
	m.editIndex = editIndex
	m.editRaw = value
	m.textInput.Prompt = "New task > "
	m.textInput.Placeholder = "(A) do a thing for +project @place due:friday"
	if mode == inputEdit {
		m.textInput.Prompt = fmt.Sprintf("Edit task %d > ", editIndex)
		m.textInput.Placeholder = ""
	}
	m.tags = knownTags(m.repo)
	m.textInput.SetValue(value)
	m.textInput.CursorEnd()
	m.textInput.Focus()
	m.refreshSuggestions()
}

// closeInput closes the task input and forgets what was in it.
func (m *model) closeInput() {
	m.input = inputClosed
	m.textInput.SetValue("")
	m.textInput.SetSuggestions(nil)
	m.textInput.Blur()
}

// refreshSuggestions offers tag completions while the cursor is at the
// end of a tag being typed.
func (m *model) refreshSuggestions() {
	value := m.textInput.Value()
	if m.textInput.Position() != len([]rune(value)) {
		m.textInput.SetSuggestions(nil)
		return
	}
	m.textInput.SetSuggestions(tagSuggestions(value, m.tags))
}

// inputView is the open task input: the text box, the line it saves
// as, and anything that stops or questions the save.
func (m model) inputView() string {
	if m.input == inputClosed {
		return ""
	}
	// The text input scrolls within its width but pads past it when it
	// shows a completion, so its line is cut to the screen plainly: what
	// overflows is padding.
	lines := []string{m.textInput.View()}
	if m.width > 0 {
		lines[0] = ansi.Truncate(lines[0], m.width, "")
	}
	d := readDraft(m.textInput.Value(), m.now(), m.input == inputAdd)
	switch {
	case d.raw != "":
		label := "  saves as "
		if len(d.problems) > 0 {
			label = "  reads as "
		}
		label = lipgloss.NewStyle().Foreground(m.styles.theme.Muted).Render(label)
		lines = append(lines, label+m.styles.line(d.raw, domain.Tokens(d.raw), nil, false))
	case m.input == inputEdit:
		d.problems = append(d.problems, "a task can't be empty (esc cancels, d deletes)")
	}
	for _, p := range d.problems {
		lines = append(lines, m.styles.status.Render("can't save: "+p))
	}
	for _, w := range d.warnings {
		lines = append(lines, m.styles.status.Render("note: "+w))
	}
	for i := 1; i < len(lines); i++ {
		if m.width > 0 {
			lines[i] = ansi.Truncate(lines[i], m.width, "…")
		}
	}
	return strings.Join(lines, "\n")
}
