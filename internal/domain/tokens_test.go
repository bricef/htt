package domain

import (
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// tokenTexts renders tokens as "kind:text" for readable comparisons.
func tokenTexts(line string, tokens []Token) []string {
	names := map[TokenKind]string{
		TokenWord: "word", TokenDone: "done", TokenPriority: "priority", TokenDate: "date",
		TokenPlusTag: "plus", TokenAtTag: "at", TokenHashTag: "hash", TokenAnnotation: "annotation",
	}
	out := make([]string, len(tokens))
	for i, tk := range tokens {
		out[i] = names[tk.Kind] + ":" + line[tk.Start:tk.End]
	}
	return out
}

func TestTokens_LabelsEachWord(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"x (A) 2026-05-20 2026-05-01 call +mum @phone #family due:2026-05-25 now",
			[]string{"done:x", "priority:(A)", "date:2026-05-20", "date:2026-05-01", "word:call",
				"plus:+mum", "at:@phone", "hash:#family", "annotation:due:2026-05-25", "word:now"}},
		{"(B)  two  spaces\tand a tab", // offsets survive runs of whitespace
			[]string{"priority:(B)", "word:two", "word:spaces", "word:and", "word:a", "word:tab"}},
		{"café +projé buy + milk note:été", // non-ASCII: offsets are bytes
			[]string{"word:café", "plus:+projé", "word:buy", "word:+", "word:milk", "annotation:note:été"}},
		{"2026-05-20 2026-05-21 (A) x", // only the preamble's date and marks count
			[]string{"date:2026-05-20", "word:2026-05-21", "word:(A)", "word:x"}},
		{"x 2026-05-20", []string{"done:x", "date:2026-05-20"}},
	}
	for _, tc := range cases {
		if got := tokenTexts(tc.line, Tokens(tc.line)); !slices.Equal(got, tc.want) {
			t.Errorf("Tokens(%q) =\n  %q\nwant\n  %q", tc.line, got, tc.want)
		}
	}
}

func TestTokens_AgreeWithTheParsedTask(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	isSpace := func(r rune) bool { return strings.ContainsRune(wordSpace, r) }
	for range 3000 {
		line := randomLine(r)
		if r.IntN(4) == 0 {
			line = " \t" + strings.ReplaceAll(line, " ", "  ") + " "
		}
		tokens := Tokens(line)

		// Every word of the line is exactly one token, in order.
		var texts []string
		for i, tk := range tokens {
			if i > 0 && tk.Start < tokens[i-1].End {
				t.Fatalf("%q: tokens overlap: %v", line, tokens)
			}
			texts = append(texts, line[tk.Start:tk.End])
		}
		if want := strings.FieldsFunc(line, isSpace); !slices.Equal(texts, want) {
			t.Fatalf("%q: token texts %q, want the words %q", line, texts, want)
		}

		// Each kind says what NewTask made of the word.
		task := mustTask(t, line)
		by := map[TokenKind][]string{}
		for _, tk := range tokens {
			by[tk.Kind] = append(by[tk.Kind], line[tk.Start:tk.End])
		}
		check := func(what string, got, want []string) {
			t.Helper()
			if !slices.Equal(got, want) {
				t.Fatalf("%q: %s tokens %q, want %q", line, what, got, want)
			}
		}
		check("done", by[TokenDone], map[bool][]string{true: {"x"}}[task.Completed])
		var priority, dates []string
		if task.Priority != "" {
			priority = []string{"(" + task.Priority + ")"}
		}
		for _, d := range []struct {
			set bool
			s   string
		}{
			{!task.CompletedOn.IsZero(), task.CompletedOn.Format("2006-01-02")},
			{!task.CreatedOn.IsZero(), task.CreatedOn.Format("2006-01-02")},
		} {
			if d.set {
				dates = append(dates, d.s)
			}
		}
		check("priority", by[TokenPriority], priority)
		check("date", by[TokenDate], dates)
		check("plus tag", by[TokenPlusTag], task.Tags["+"])
		check("at tag", by[TokenAtTag], task.Tags["@"])
		check("hash tag", by[TokenHashTag], task.Tags["#"])

		annotations := map[string]string{}
		for _, a := range by[TokenAnnotation] {
			k, v, _ := strings.Cut(a, ":")
			annotations[k] = v
		}
		if !maps.Equal(annotations, task.Annotations) {
			t.Fatalf("%q: annotation tokens %v, want %v", line, annotations, task.Annotations)
		}

		var entry []string
		for _, tk := range tokens {
			switch tk.Kind {
			case TokenWord, TokenPlusTag, TokenAtTag, TokenHashTag:
				entry = append(entry, line[tk.Start:tk.End])
			}
		}
		if got := strings.Join(entry, " "); got != task.Entry() {
			t.Fatalf("%q: text tokens %q, want the entry %q", line, got, task.Entry())
		}
	}
}

func BenchmarkTokens(b *testing.B) {
	line := "(A) 2026-05-20 call +mum @phone about the #garden and the long list of things due:2026-05-25"
	for b.Loop() {
		_ = Tokens(line)
	}
}
