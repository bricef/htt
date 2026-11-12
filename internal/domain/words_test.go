package domain

import (
	"errors"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

// The parser reads a line as whitespace-separated words. A word is never
// split, and every word lands somewhere: the preamble, the entry or an
// annotation. Rewriting a task (a priority change, completion) rebuilds
// its line from those parts, so a word the parser drops or splits is a
// word lost from the user's file.

func TestNewTask_KeepsEveryWord(t *testing.T) {
	cases := []struct {
		raw         string
		entry       string
		tags        map[string][]string
		annotations map[string]string
	}{
		// A lone +, @ or # used to stop the parse: "milk" was dropped.
		{raw: "buy + milk", entry: "buy + milk"},
		{raw: "fix A # B @ C", entry: "fix A # B @ C"},
		// Non-ASCII letters used to end a word: "caf é", tag "+proj".
		{raw: "café +projé @maison", entry: "café +projé @maison",
			tags: map[string][]string{"+": {"+projé"}, "@": {"@maison"}}},
		// An annotation value used to stop at the first non-ASCII
		// character or backtick, leaving the rest as a separate word.
		{raw: "later note:été", entry: "later", annotations: map[string]string{"note": "été"}},
		{raw: "foo:bar`baz x", entry: "x", annotations: map[string]string{"foo": "bar`baz"}},
		// A created date must be a whole word.
		{raw: "2026-05-20x task", entry: "2026-05-20x task"},
		// A lone sigil or a non-ASCII tag as the first word used to
		// crash NewTask (nil parse tree): a "# notes" line in a todo
		// file stopped every command.
		{raw: "+ call mom", entry: "+ call mom"},
		{raw: "# notes", entry: "# notes"},
		{raw: "x #ü done", entry: "#ü done", tags: map[string][]string{"#": {"#ü"}}},
		// So did a line that is all preamble.
		{raw: "2026-05-20", entry: ""},
		{raw: "x 2026-05-21 2026-05-20", entry: ""},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			task := mustTask(t, tc.raw)
			if task.Entry() != tc.entry {
				t.Errorf("entry = %q, want %q", task.Entry(), tc.entry)
			}
			for sigil, want := range tc.tags {
				if got := task.Tags[sigil]; !slices.Equal(got, want) {
					t.Errorf("tags %s = %q, want %q", sigil, got, want)
				}
			}
			want := tc.annotations
			if want == nil {
				want = map[string]string{}
			}
			if !maps.Equal(task.Annotations, want) {
				t.Errorf("annotations = %v, want %v", task.Annotations, want)
			}
		})
	}
}

// wordShapes are the kinds of word a todo.txt line holds, including the
// awkward ones: lone sigils, non-ASCII text, colons in odd places,
// characters outside printable ASCII, invalid and run-on dates.
var wordShapes = []string{
	"buy", "milk", "x", "X", "(A)x", "(B)", "(b)", "2+2", "a`b", "it's",
	"café", "naïve", "日本語", "🙂", "a b", "nb ", " ", "a\fb",
	"+", "@", "#", "+proj", "@home", "#tag", "+projé", "@a@b", "#ü", "+a+b",
	"due:2026-05-20", "http://x.com/a?b=c", "note:été", "a:b:c", "k:", ":v",
	"foo:bar`baz", "t:10:30", "clé:valeur",
	"2026-05-20", "2026-13-45", "2026-05-20x",
}

// randomLine builds a line from wordShapes, sometimes behind a
// preamble. Annotation keys are kept unique: a repeated key is a
// separate known limit (Annotations is a map).
func randomLine(r *rand.Rand) string {
	var words []string
	if r.IntN(3) == 0 {
		words = append(words, "x")
	}
	if r.IntN(3) == 0 {
		words = append(words, "(B)")
	}
	for range r.IntN(3) {
		words = append(words, "2026-01-0"+string(rune('1'+r.IntN(9))))
	}
	keys := map[string]bool{}
	for range 1 + r.IntN(7) {
		w := wordShapes[r.IntN(len(wordShapes))]
		if k, v, ok := strings.Cut(w, ":"); ok && k != "" && v != "" && !strings.ContainsAny(k[:1], "+@#") {
			if keys[k] {
				continue
			}
			keys[k] = true
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

// wordCounts is the multiset of a line's words, split on the parser's
// whitespace only (strings.Fields would also split on U+00A0).
func wordCounts(line string) map[string]int {
	out := map[string]int{}
	for _, w := range strings.FieldsFunc(line, func(r rune) bool { return strings.ContainsRune(wordSpace, r) }) {
		out[w]++
	}
	return out
}

func TestRewrite_KeepsEveryWord(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 3000 {
		raw := randomLine(r)
		task, err := NewTask(raw)
		if err != nil {
			t.Fatalf("NewTask(%q): %v", raw, err)
		}
		before := *task
		want := wordCounts(raw)
		if task.Priority != "" {
			want["("+task.Priority+")"]--
		}
		want["(A)"]++
		maps.DeleteFunc(want, func(_ string, n int) bool { return n == 0 })

		task.setPriority("A")
		if got := wordCounts(task.Raw); !maps.Equal(got, want) {
			t.Fatalf("setting priority A rewrote\n  %q\nas\n  %q\nwords lost or changed", raw, task.Raw)
		}

		// The rewrite reads back as the same task.
		again, err := NewTask(task.Raw)
		if err != nil {
			t.Fatalf("NewTask(%q): %v", task.Raw, err)
		}
		// Known limit (TODO.md): rebuild moves annotations to the end, so
		// a date word that followed one ("due:x 2026-05-20 buy") ends up
		// first in the body and reads back as the created date. No word
		// is lost, which the check above covers.
		first, _, _ := strings.Cut(before.Entry(), " ")
		if _, isDate := ParseLocalDate(first); isDate && before.CreatedOn.IsZero() {
			continue
		}
		if again.Priority != "A" || again.Completed != before.Completed ||
			!again.CompletedOn.Equal(before.CompletedOn) || !again.CreatedOn.Equal(before.CreatedOn) ||
			again.Entry() != before.Entry() || !maps.Equal(again.Annotations, before.Annotations) ||
			!maps.EqualFunc(again.Tags, before.Tags, slices.Equal) {
			t.Fatalf("%q rewritten as %q reads back differently:\n  before %+v\n  after  %+v", raw, task.Raw, before, *again)
		}
	}
}

func TestComplete_KeepsEveryWord(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	when := time.Date(2026, 5, 22, 9, 0, 0, 0, time.Local)
	for range 3000 {
		raw := randomLine(r)
		task, err := NewTask(raw)
		if err != nil {
			t.Fatalf("NewTask(%q): %v", raw, err)
		}
		if task.Completed {
			continue
		}
		want := wordCounts(raw)
		want["x"]++
		want["2026-05-22"]++
		want["context:todo"]++

		if _, err := task.markCompleted(&Context{Name: "todo"}, when); err != nil {
			t.Fatalf("markCompleted(%q): %v", raw, err)
		}
		if got := wordCounts(task.Raw); !maps.Equal(got, want) {
			t.Fatalf("completing\n  %q\nwrote\n  %q\nwords lost or changed", raw, task.Raw)
		}
	}
}

func TestAnnotate_RejectsInvalidValue(t *testing.T) {
	task := mustTask(t, "hello")
	// A value with whitespace would read back as two words.
	for _, v := range []string{"bad value", "", "tab\there"} {
		if _, err := task.Annotate("ok", v); !errors.Is(err, ErrInvalidAnnotationValue) {
			t.Errorf("Annotate(ok,%q) err = %v, want ErrInvalidAnnotationValue", v, err)
		}
	}
}

func TestAnnotate_AcceptsAnyWordAsValue(t *testing.T) {
	// Completing a task annotates it with its context's name, so a
	// context named "café" must be a valid value.
	for _, v := range []string{"café", "bar`baz", "2026-05-20T10:00:00+01:00"} {
		task := mustTask(t, "hello")
		if _, err := task.Annotate("ok", v); err != nil {
			t.Errorf("Annotate(ok,%q): %v", v, err)
			continue
		}
		if task.Raw != "hello ok:"+v {
			t.Errorf("Raw = %q, want %q", task.Raw, "hello ok:"+v)
		}
	}
}
