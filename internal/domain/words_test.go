package domain

import (
	"errors"
	"fmt"
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

func TestNewTask_TimesAndLinksAreText(t *testing.T) {
	// A key:value annotation is extra information about the task, and
	// rewriting a task moves annotations to the end of the line. Times
	// and links have the same word:word shape but are part of what the
	// task says, so they're words and stay where they were written.
	text := []string{
		"10:30", "9:05", "10:30:15", "12:30pm", "3:16", "16:9",
		"https://example.com/a?b=c", "http://x.io", "HTTPS://Example.com", "ftp://files.example.com",
		"ssh://git@host:22/repo", "file:///home/me/notes.txt", "git+ssh://host/repo",
		"mailto:bob@example.com", "MAILTO:Bob@Example.com", "tel:+441234567890", "sms:+441234567890", "magnet:?xt=urn:btih:abc",
	}
	for _, word := range text {
		raw := "call Bob at " + word + " about the report"
		task := mustTask(t, raw)
		if len(task.Annotations) != 0 || task.Entry() != raw {
			t.Errorf("%q: annotations %v, entry %q; want %q read as text", raw, task.Annotations, task.Entry(), word)
		}
		if got := task.setPriority("A").Raw; got != "(A) "+raw {
			t.Errorf("setting a priority rewrote %q as %q; the %s should stay in place", raw, got, word)
		}
	}

	annotations := map[string]string{
		"due:2026-05-20": "due", "context:todo": "context", "timestamp:2026-05-15T10:30:00+01:00": "timestamp",
		"url:https://example.com": "url", "v1:2": "v1", "2fa:on": "2fa", "data:x": "data",
	}
	for word, key := range annotations {
		task := mustTask(t, "task "+word)
		if _, ok := task.Annotations[key]; !ok || task.Entry() != "task" {
			t.Errorf("%q should stay an annotation (key %q): annotations %v, entry %q", word, key, task.Annotations, task.Entry())
		}
	}
}

func TestAnnotate_RefusesWhatWouldReadBackAsText(t *testing.T) {
	// Annotate must not write an annotation the parser reads back as a
	// word: it would vanish from Annotations on the next load.
	for _, kv := range [][2]string{{"10", "30"}, {"https", "//example.com"}, {"mailto", "bob@example.com"}} {
		if _, err := mustTask(t, "hello").Annotate(kv[0], kv[1]); !errors.Is(err, ErrInvalidAnnotationKey) {
			t.Errorf("Annotate(%q, %q) err = %v, want ErrInvalidAnnotationKey", kv[0], kv[1], err)
		}
	}
	if _, err := mustTask(t, "hello").Annotate("url", "https://example.com"); err != nil {
		t.Errorf("a link as an annotation's value is fine: %v", err)
	}
}

func TestNewTask_ManyTagsAndAnnotations(t *testing.T) {
	// More than 100 of one kind used to hang NewTask (parseutils.Query).
	var b strings.Builder
	b.WriteString("many")
	for i := range 150 {
		fmt.Fprintf(&b, " @c%d +p%d #h%d k%d:v", i, i, i, i)
	}
	done := make(chan *Task)
	go func() {
		task, err := NewTask(b.String())
		if err != nil {
			t.Errorf("NewTask: %v", err)
		}
		done <- task
	}()
	select {
	case task := <-done:
		if task == nil {
			return
		}
		for _, sigil := range []string{"@", "+", "#"} {
			if n := len(task.Tags[sigil]); n != 150 {
				t.Errorf("%d %s tags, want 150", n, sigil)
			}
		}
		if n := len(task.Annotations); n != 150 {
			t.Errorf("%d annotations, want 150", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("NewTask did not return within 5s")
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
	"10:30", "12:30pm", "https://example.com/x", "mailto:bob@example.com", "url:https://x.io",
	"2026-05-20", "2026-13-45", "2026-05-20x",
}

// randomLine builds a line from wordShapes, sometimes behind a
// preamble. Words, annotations among them, may repeat.
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
	for range 1 + r.IntN(7) {
		words = append(words, wordShapes[r.IntN(len(wordShapes))])
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

func TestStampCreated_DatesOnlyAnUndatedTask(t *testing.T) {
	day := time.Date(2026, 5, 22, 9, 30, 0, 0, time.Local)
	cases := []struct{ raw, want string }{
		{"buy milk", "2026-05-22 buy milk"},
		{"(A) call due:friday mum", "(A) 2026-05-22 call due:friday mum"},
		{"2026-01-01 dated already", "2026-01-01 dated already"},
		{"(B) 2026-01-01 dated already", "(B) 2026-01-01 dated already"},
	}
	for _, tc := range cases {
		if got := mustTask(t, tc.raw).StampCreated(day).Raw; got != tc.want {
			t.Errorf("StampCreated(%q) = %q, want %q", tc.raw, got, tc.want)
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
