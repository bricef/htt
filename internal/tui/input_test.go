package tui

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/dates"
	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// day formats the date a phrase names on listNow (Saturday 2026-05-16),
// as dates.ParsePhrase reads it: the oracle for due resolution.
func day(t *testing.T, phrase string) string {
	t.Helper()
	d, err := dates.ParsePhrase(phrase, listNow)
	if err != nil {
		t.Fatalf("ParsePhrase(%q): %v", phrase, err)
	}
	return d.Format("2006-01-02")
}

func TestResolveDue_TurnsPhrasesIntoDates(t *testing.T) {
	cases := []struct{ in, want string }{
		{"call mum due:friday", "call mum due:" + day(t, "friday")},
		{`call due:"in two weeks" mum`, "call due:" + day(t, "in two weeks") + " mum"},
		{"due:tomorrow first", "due:" + day(t, "tomorrow") + " first"},
		{"due:next-friday", "due:" + day(t, "next friday")},
		{"due:in_two_weeks", "due:" + day(t, "in two weeks")},
		{"a due:friday b due:tomorrow", "a due:" + day(t, "friday") + " b due:" + day(t, "tomorrow")},
		{"due:2026-06-01 stays", "due:2026-06-01 stays"},
		{"overdue:friday Due:friday are other keys", "overdue:friday Due:friday are other keys"},
		{"no due date at all", "no due date at all"},
	}
	for _, tc := range cases {
		got, problems := resolveDue(tc.in, listNow)
		if got != tc.want || len(problems) != 0 {
			t.Errorf("resolveDue(%q) = %q, %q; want %q and no problems", tc.in, got, problems, tc.want)
		}
	}
	if day(t, "friday") != "2026-05-22" {
		t.Fatalf("the oracle should read friday as 2026-05-22 from Saturday 2026-05-16")
	}
}

func TestResolveDue_ReportsWhatItCantRead(t *testing.T) {
	cases := []struct{ in, problem string }{
		{"call due:blorp", "can't tell what date due:blorp means"},
		{`call due:"in two weeks`, `due:"in two weeks needs a closing quote`},
		{`call due:"`, `due:" needs a closing quote`},
		{"call due:", "due: needs a date"},
		{"call due:2026-13-45", "due:2026-13-45 isn't a real date"},
	}
	for _, tc := range cases {
		got, problems := resolveDue(tc.in, listNow)
		if len(problems) != 1 || !strings.Contains(problems[0], tc.problem) {
			t.Errorf("resolveDue(%q) problems = %q, want one containing %q", tc.in, problems, tc.problem)
		}
		if got != tc.in {
			t.Errorf("resolveDue(%q) changed the text to %q; an unread value stays as typed", tc.in, got)
		}
	}
}

func TestReadDraft_IsTheLineASaveWrites(t *testing.T) {
	cases := []struct {
		value  string
		adding bool
		want   string
	}{
		// A new task gets today's created date, and the line is rebuilt
		// (annotations last) as Context.AddTask saves it.
		{"call due:friday mum", true, "2026-05-16 call mum due:2026-05-22"},
		{"(A) call +mum", true, "(A) 2026-05-16 call +mum"},
		{"2026-01-01 dated already", true, "2026-01-01 dated already"},
		// An edit saves the line as typed, due phrases resolved.
		{"call due:friday mum", false, "call due:2026-05-22 mum"},
		{"  padded  ", false, "padded"},
		{"   ", true, ""},
	}
	for _, tc := range cases {
		d := readDraft(tc.value, listNow, tc.adding)
		if d.raw != tc.want || len(d.problems) != 0 {
			t.Errorf("readDraft(%q, adding=%v) = %q %q, want %q", tc.value, tc.adding, d.raw, d.problems, tc.want)
		}
	}
	if d := readDraft("call due:blorp", listNow, true); len(d.problems) == 0 || d.raw != "call due:blorp" {
		t.Errorf("a draft with a problem keeps the text as typed and reports it, got %+v", d)
	}
}

func TestWordWarnings(t *testing.T) {
	cases := []struct {
		raw    string
		adding bool
		want   []string
	}{
		{"(A) 2026-05-16 call +mum @home #x due:2026-05-22", true, nil},
		{"(a) call mum", false, []string{"(a) is text: priorities are capitals, (A)"}},
		{"call (B) mum", false, []string{"(B) is text: a priority goes at the start"}},
		{"(D) call mum", false, []string{"(D): htt's priorities are (A), (B) and (C)"}},
		{"x call mum", true, []string{`"x" at the start marks the task done`}},
		{"x 2026-05-16 call mum", false, nil}, // editing a done task's line is fine
		{"2026-13-45 call mum", false, []string{"2026-13-45 isn't a real date, so it's text"}},
	}
	for _, tc := range cases {
		if got := wordWarnings(tc.raw, tc.adding); !slices.Equal(got, tc.want) {
			t.Errorf("wordWarnings(%q, %v) = %q, want %q", tc.raw, tc.adding, got, tc.want)
		}
	}
}

func TestTagSuggestions_CompleteTheLastWord(t *testing.T) {
	tags := map[string][]string{"+": {"+garden", "+gym"}, "@": {"@home"}}
	cases := []struct {
		value string
		want  []string
	}{
		{"plan +g", []string{"plan +garden", "plan +gym"}},
		{"plan +", []string{"plan +garden", "plan +gym"}},
		{"@", []string{"@home"}},
		{"plan +garden", []string{"plan +gym"}}, // the input filters by prefix: nothing left
		{"plan #", nil},
		{"plan ", nil},
		{"plan", nil},
		{"", nil},
	}
	for _, tc := range cases {
		if got := tagSuggestions(tc.value, tags); !slices.Equal(got, tc.want) {
			t.Errorf("tagSuggestions(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// seedRepo builds a memory repo with the given contexts' lines.
func seedRepo(t *testing.T, contexts map[string][]string) *storage.MemoryRepository {
	t.Helper()
	repo := storage.NewMemoryRepository()
	for _, name := range slices.Sorted(maps.Keys(contexts)) {
		ctx, err := repo.Context(name)
		if err != nil {
			t.Fatalf("Context(%q): %v", name, err)
		}
		for _, line := range contexts[name] {
			task, err := domain.NewTask(line)
			if err != nil {
				t.Fatalf("NewTask(%q): %v", line, err)
			}
			ctx.Tasks = append(ctx.Tasks, task)
		}
		if err := repo.Save(ctx); err != nil {
			t.Fatalf("Save(%q): %v", name, err)
		}
	}
	return repo
}

func TestKnownTags_AcrossContextsMostUsedFirst(t *testing.T) {
	repo := seedRepo(t, map[string][]string{
		"todo":                    {"a +gym", "b +garden @home", "c +garden"},
		"work":                    {"d +garden @office #idea"},
		domain.DoneContextName:    {"x e @home @office @office"},
		domain.ArchiveContextName: {"f +deleted"},
	})
	got := knownTags(repo)
	want := map[string][]string{
		"+": {"+garden", "+gym"},
		"@": {"@office", "@home"},
		"#": {"#idea"},
	}
	for sigil, tags := range want {
		if !slices.Equal(got[sigil], tags) {
			t.Errorf("tags %s = %q, want %q (archive left out)", sigil, got[sigil], tags)
		}
	}
}

// inputModel is a model over repo in the todo context, ready to type
// into: a steady cursor and a fixed clock.
func inputModel(t *testing.T, repo *storage.MemoryRepository) model {
	t.Helper()
	ctx, err := repo.Context("todo")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if err := repo.SetCurrent("todo"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	m := Model(repo, ctx, Options{})
	m.now = func() time.Time { return listNow }
	m.textInput.Cursor.SetMode(cursor.CursorStatic)
	m.width, m.height = 100, 30
	m.layout()
	return m
}

func send(t *testing.T, m model, msgs ...tea.Msg) model {
	t.Helper()
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = asModel(t, next)
	}
	return m
}

func typed(s string) []tea.Msg {
	var out []tea.Msg
	for _, r := range s {
		out = append(out, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return out
}

var (
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	esc   = tea.KeyMsg{Type: tea.KeyEsc}
	tab   = tea.KeyMsg{Type: tea.KeyTab}
	ctrlN = tea.KeyMsg{Type: tea.KeyCtrlN}
)

func stored(t *testing.T, repo *storage.MemoryRepository, name string) []string {
	t.Helper()
	return (&storageRepo{repo}).raws(t, name)
}

func TestEdit_SavesTheTaskInPlace(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"(A) a", "b +x", "c"}})
	m := inputModel(t, repo)
	m.list.Select(1)

	m = send(t, m, keyPress("e"))
	if m.input != inputEdit || m.textInput.Value() != "b +x" {
		t.Fatalf("e should open the selected task for editing; input=%v value=%q", m.input, m.textInput.Value())
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Edit task 1 > b +x") {
		t.Errorf("the input should say which task it edits:\n%s", view)
	}
	m = send(t, m, typed(" due:friday")...)
	m = send(t, m, enter)

	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"(A) a", "b +x due:2026-05-22", "c"}) {
		t.Errorf("todo = %q, want task 1 replaced in place with its due date resolved", got)
	}
	if it, _ := m.selectedTask(); it.index != 1 || m.input != inputClosed {
		t.Errorf("after saving, the input closes and the cursor stays on task 1 (on %d, input %v)", it.index, m.input)
	}
}

func TestEdit_EscCancelsAndUnchangedSavesNothing(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"a", "b"}})
	m := inputModel(t, repo)
	m = send(t, m, keyPress("e"))
	m = send(t, m, typed(" more")...)
	m = send(t, m, esc)
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"a", "b"}) || m.input != inputClosed {
		t.Errorf("esc should cancel the edit: todo=%q input=%v", got, m.input)
	}
	m = send(t, m, keyPress("e"), enter)
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"a", "b"}) || m.input != inputClosed {
		t.Errorf("saving an unchanged task should change nothing: todo=%q input=%v", got, m.input)
	}
}

func TestEdit_AProblemKeepsTheInputOpen(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"a"}})
	m := inputModel(t, repo)
	m = send(t, m, keyPress("e"))
	m = send(t, m, typed(" due:blorp")...)
	m = send(t, m, enter)

	if m.input != inputEdit || m.textInput.Value() != "a due:blorp" {
		t.Errorf("a problem should keep the edit open as typed; input=%v value=%q", m.input, m.textInput.Value())
	}
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"a"}) {
		t.Errorf("nothing should be saved, todo=%q", got)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "can't save: can't tell what date due:blorp means") {
		t.Errorf("the view should say why it can't save:\n%s", view)
	}
	if !strings.Contains(view, "reads as a due:blorp") || strings.Contains(view, "saves as") {
		t.Errorf("a draft that can't be saved shouldn't claim what it saves as:\n%s", view)
	}

	// Emptying the task is a problem too.
	for range len("a due:blorp") {
		m = send(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m = send(t, m, enter)
	if m.input != inputEdit || !strings.Contains(ansi.Strip(m.View()), "a task can't be empty") {
		t.Errorf("an empty edit should stay open and say so:\n%s", ansi.Strip(m.View()))
	}
}

func TestEdit_RefusedInTheDoneTabAndWithNoTask(t *testing.T) {
	repo := seedRepo(t, map[string][]string{domain.DoneContextName: {"x 2026-05-15 done"}, "todo": {}})
	m := inputModel(t, repo)
	m = send(t, m, keyPress("e"))
	if m.input != inputClosed {
		t.Errorf("with no task selected, e should do nothing")
	}

	ctx, _ := repo.Context(domain.DoneContextName)
	m.context = ctx
	m.showTasks(-1)
	m = send(t, m, keyPress("e"))
	if m.input != inputClosed || !strings.Contains(m.status, "done tab can't be edited") {
		t.Errorf("e in the done tab should explain itself (status %q), input %v", m.status, m.input)
	}
	m = send(t, m, keyPress("n"))
	if m.input != inputClosed || !strings.Contains(m.status, "not the done tab") {
		t.Errorf("n in the done tab should explain itself (status %q), input %v", m.status, m.input)
	}
}

func TestEdit_ATaskChangedUnderneathIsNotOverwritten(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"a", "b"}})
	m := inputModel(t, repo)
	m = send(t, m, keyPress("e"))
	m = send(t, m, typed(" edited")...)
	// The task the edit started from is no longer at its index.
	other, _ := domain.NewTask("someone else's change")
	m.context.Tasks[0] = other

	m = send(t, m, enter)
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("todo = %q; the edit must not overwrite a task that changed", got)
	}
	if !strings.Contains(m.status, "changed while you were editing") {
		t.Errorf("status = %q, want an explanation", m.status)
	}
}

func TestAdd_ResolvesDueAndShowsThePreview(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"a"}})
	m := inputModel(t, repo)
	m = send(t, m, keyPress("n"))
	m = send(t, m, typed("(A) call +mum due:friday")...)

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "saves as (A) 2026-05-16 call +mum due:2026-05-22") {
		t.Errorf("the preview should show the line as it will be saved:\n%s", view)
	}
	m = send(t, m, enter)
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"a", "(A) 2026-05-16 call +mum due:2026-05-22"}) {
		t.Errorf("todo = %q, want the previewed line", got)
	}
	if it, _ := m.selectedTask(); it.index != 1 || m.input != inputClosed {
		t.Errorf("the new task should be selected and the input closed (on %d, input %v)", it.index, m.input)
	}
}

func TestAdd_AProblemKeepsWhatWasTyped(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"a"}})
	m := inputModel(t, repo)
	m = send(t, m, keyPress("n"))
	m = send(t, m, typed(`call due:"in two weeks`)...)
	m = send(t, m, enter)
	if m.input != inputAdd || m.textInput.Value() != `call due:"in two weeks` {
		t.Errorf("the input should stay open as typed; input=%v value=%q", m.input, m.textInput.Value())
	}
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"a"}) {
		t.Errorf("nothing should be saved, todo=%q", got)
	}
	m = send(t, m, typed(`"`)...)
	m = send(t, m, enter)
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"a", "2026-05-16 call due:" + day(t, "in two weeks")}) {
		t.Errorf("closing the quote should let it save, todo=%q", got)
	}
}

func TestInput_TabCompletesTagsFromEveryContext(t *testing.T) {
	repo := seedRepo(t, map[string][]string{
		"todo":                 {"a +garden", "b +garden"},
		"work":                 {"c +gym"},
		domain.DoneContextName: {"x d @home"},
	})
	m := inputModel(t, repo)
	m = send(t, m, keyPress("n"))
	m = send(t, m, typed("plan +g")...)
	// The completion shows greyed after the text, and the input line
	// still fits the screen without being marked as cut.
	for _, line := range strings.Split(m.View(), "\n") {
		if plain := ansi.Strip(line); strings.HasPrefix(plain, "New task > ") {
			if !strings.Contains(plain, "plan +garden") || strings.Contains(plain, "…") || ansi.StringWidth(line) > m.width {
				t.Errorf("input line with a completion: %q", plain)
			}
		}
	}
	m = send(t, m, tab)
	if got := m.textInput.Value(); got != "plan +garden" {
		t.Errorf("tab should complete the most used tag: %q", got)
	}
	m = send(t, m, typed(" @H")...)
	m = send(t, m, tab)
	if got := m.textInput.Value(); got != "plan +garden @Home" {
		t.Errorf("completion is case-insensitive and keeps what was typed: %q", got)
	}
	m = send(t, m, typed(" +g")...)
	m = send(t, m, ctrlN, tab)
	if got := m.textInput.Value(); got != "plan +garden @Home +gym" {
		t.Errorf("ctrl+n should move to the next tag: %q", got)
	}
	// With the cursor back inside a tag being typed, nothing is
	// offered: Tab would complete at the end, away from the cursor.
	m = send(t, m, typed(" +g")...)
	m = send(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if len(m.textInput.MatchedSuggestions()) != 0 {
		t.Errorf("no completion while the cursor isn't at the end: %q", m.textInput.MatchedSuggestions())
	}
	m = send(t, m, tab)
	if got := m.textInput.Value(); got != "plan +garden @Home +gym +g" {
		t.Errorf("tab mid-text should change nothing: %q", got)
	}
}

func TestInput_HelpBarShowsTheInputsKeys(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"a"}})
	m := inputModel(t, repo)
	if view := ansi.Strip(m.View()); strings.Contains(view, "tab complete tag") {
		t.Errorf("the input's keys shouldn't show while it's closed")
	}
	m = send(t, m, keyPress("n"))
	view := ansi.Strip(m.View())
	for _, want := range []string{"enter save", "esc cancel", "tab complete tag"} {
		if !strings.Contains(view, want) {
			t.Errorf("help bar should show %q while typing:\n%s", want, view)
		}
	}
}

func TestInput_FitsANarrowScreen(t *testing.T) {
	repo := seedRepo(t, map[string][]string{"todo": {"a"}})
	m := inputModel(t, repo)
	m.width = 40
	m = send(t, m, keyPress("n"))
	m = send(t, m, typed(strings.Repeat("long words ", 8)+"due:blorp")...)
	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Errorf("line is %d wide on a 40-wide screen: %q", w, ansi.Strip(line))
		}
	}
	// The input scrolls to keep the cursor, at the end, in view.
	if !regexp.MustCompile(`(?m)^New task > .*due:blorp`).MatchString(ansi.Strip(view)) {
		t.Errorf("the input line should scroll to show the end of the text:\n%s", ansi.Strip(view))
	}
}
