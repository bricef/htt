package domain

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// testCategories is the plan's example config: a break category that
// doesn't count as work, then two that do.
func testCategories() []Category {
	return []Category{
		{Name: "break", Tags: []string{"@break", "@lunch"}, Work: false},
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
		{Name: "admin", Tags: []string{"@email", "+admin"}, Work: true},
	}
}

func mustCategoriser(t *testing.T, categories []Category) *Categoriser {
	t.Helper()
	c, err := NewCategoriser(categories)
	if err != nil {
		t.Fatalf("NewCategoriser: %v", err)
	}
	return c
}

func TestCategoriser_FirstMatchingCategoryWins(t *testing.T) {
	c := mustCategoriser(t, testCategories())
	cases := []struct {
		entry string
		want  string
	}{
		{"@lunch", "break"},
		{"@break coffee", "break"},
		{"reviewing the auth PR +htt", "htt"},
		{"mail @email", "admin"},
		{"budget +admin", "admin"},
		// Several tags: the first category in the list with one wins,
		// whatever order the tags are written in.
		{"+htt @lunch", "break"},
		{"@email +htt", "htt"},
		{"+admin +htt @break", "break"},
		// No category's tag: the default category.
		{"reading", DefaultCategoryName},
		{"@start", DefaultCategoryName},
		{"+other @phone #deep", DefaultCategoryName},
	}
	for _, tc := range cases {
		got := c.Categorise(mustTask(t, tc.entry))
		if got.Name != tc.want {
			t.Errorf("Categorise(%q) = %q, want %q", tc.entry, got.Name, tc.want)
		}
	}
}

func TestCategoriser_MatchSaysWhetherACategoryMatched(t *testing.T) {
	// Match gives the category and whether one of its tags matched; an
	// entry that matched none is in the default category, unmatched.
	plain := mustCategoriser(t, testCategories())
	onBreak, err := plain.WithDefault("break")
	if err != nil {
		t.Fatal(err)
	}
	// The default defined in the list with tags of its own: its tags
	// match like any category's, first match winning, and it is still
	// where unmatched entries go.
	deskLast := mustCategoriser(t, []Category{
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
		{Name: "working", Tags: []string{"@desk"}, Work: false},
	})
	deskFirst := mustCategoriser(t, []Category{
		{Name: "working", Tags: []string{"@desk"}, Work: true},
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
	})
	idle, err := mustCategoriser(t, []Category{
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
		{Name: "idle", Work: false},
	}).WithDefault("idle")
	if err != nil {
		t.Fatal(err)
	}
	var none *Categoriser

	cases := []struct {
		name    string
		c       *Categoriser
		entry   string
		want    string
		work    bool
		matched bool
	}{
		{"a category's tag", plain, "@lunch", "break", false, true},
		{"first match wins", plain, "+htt @lunch", "break", false, true},
		{"first match wins, whatever the tags' order", plain, "@email +htt", "htt", true, true},
		{"no tag: the built-in default", plain, "reading the news", "working", true, false},
		{"a near miss is no match", plain, "+HTT", "working", true, false},
		{"@start is ordinary", plain, "@start", "working", true, false},
		{"another default: unmatched", onBreak, "reading", "break", false, false},
		{"another default: its own tag", onBreak, "@lunch", "break", false, true},
		{"the default's own tag", deskLast, "filing @desk", "working", false, true},
		{"unmatched, in the list's working", deskLast, "reading", "working", false, false},
		{"an earlier category beats the default's tag", deskLast, "@desk +htt", "htt", true, true},
		{"the default listed first wins over a later tag", deskFirst, "+htt @desk", "working", true, true},
		{"a tagless default is never matched", idle, "reading", "idle", false, false},
		{"no categoriser", none, "+htt", "working", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, matched := tc.c.Match(mustTask(t, tc.entry))
			if got.Name != tc.want || got.Work != tc.work || matched != tc.matched {
				t.Errorf("Match(%q) = %s (work %v), matched %v; want %s (work %v), matched %v",
					tc.entry, got.Name, got.Work, matched, tc.want, tc.work, tc.matched)
			}
			if cat := tc.c.Categorise(mustTask(t, tc.entry)); cat.Name != got.Name || cat.Work != got.Work {
				t.Errorf("Categorise(%q) = %+v, Match gave %+v", tc.entry, cat, got)
			}
		})
	}
}

func TestCategoriser_Default(t *testing.T) {
	plain := mustCategoriser(t, testCategories())
	onBreak, err := plain.WithDefault("break")
	if err != nil {
		t.Fatal(err)
	}
	listed := mustCategoriser(t, []Category{{Name: "working", Tags: []string{"@desk"}, Work: false}})
	var none *Categoriser
	for _, tc := range []struct {
		name string
		c    *Categoriser
		want string
		work bool
	}{
		{"no categoriser", none, "working", true},
		{"the built-in working", plain, "working", true},
		{"a category named in WithDefault", onBreak, "break", false},
		{"the list's working", listed, "working", false},
	} {
		if got := tc.c.Default(); got.Name != tc.want || got.Work != tc.work {
			t.Errorf("%s: Default() = %+v, want %s (work %v)", tc.name, got, tc.want, tc.work)
		}
	}
}

func TestCategoriser_MatchingIsExact(t *testing.T) {
	// A configured tag matches only the identical tag in the entry:
	// same characters, same case. No prefix, partial, fuzzy or
	// case-insensitive match.
	c := mustCategoriser(t, testCategories())
	for _, entry := range []string{
		"@Break",          // case
		"@BREAK",          // case
		"@breakfast",      // prefix of the entry's tag
		"@break-time",     // prefix, with punctuation
		"@break, then +x", // punctuation is part of the tag
		"+break",          // same word, another sigil
		"#break",          // same word, another sigil
		"break",           // the bare word isn't a tag
		"a coffee break",  // nor in a sentence
		"@brea",           // partial
		"+HTT",            // case
		"+htt.",           // punctuation
		"@emails",         // plural
	} {
		if got := c.Categorise(mustTask(t, entry)); got.Name != DefaultCategoryName {
			t.Errorf("Categorise(%q) = %q, want the default category: matching is exact", entry, got.Name)
		}
	}
	if got := c.Categorise(mustTask(t, "@break coffee")); got.Name != "break" {
		t.Errorf("Categorise(%q) = %q, want break", "@break coffee", got.Name)
	}
}

func TestCategoriser_CategoryCarriesItsWorkSetting(t *testing.T) {
	c := mustCategoriser(t, testCategories())
	if got := c.Categorise(mustTask(t, "@lunch")); got.Work {
		t.Errorf("break counts as work; it was configured work: false")
	}
	if got := c.Categorise(mustTask(t, "+htt")); !got.Work {
		t.Errorf("htt should count as work")
	}
	if got := c.Categorise(mustTask(t, "anything")); !got.Work || got.Name != DefaultCategoryName {
		t.Errorf("the built-in default = %+v, want working, counting as work", got)
	}
}

func TestCategoriser_NilAndEmptyPutEverythingInWorking(t *testing.T) {
	var none *Categoriser
	empty := mustCategoriser(t, nil)
	for _, c := range []*Categoriser{none, empty} {
		for _, entry := range []string{"@lunch", "+htt", "@start", "plain"} {
			got := c.Categorise(mustTask(t, entry))
			if got.Name != DefaultCategoryName || !got.Work {
				t.Errorf("Categorise(%q) = %+v, want working, counting as work", entry, got)
			}
		}
		if names := categoryNames(c.Categories()); !slices.Equal(names, []string{DefaultCategoryName}) {
			t.Errorf("Categories() = %v, want [working]", names)
		}
	}
}

func TestCategoriser_CategoriesInConfigOrderThenTheDefault(t *testing.T) {
	c := mustCategoriser(t, testCategories())
	want := []string{"break", "htt", "admin", DefaultCategoryName}
	if got := categoryNames(c.Categories()); !slices.Equal(got, want) {
		t.Errorf("Categories() = %v, want %v", got, want)
	}
	// A default that is one of the categories isn't listed twice.
	d, err := c.WithDefault("htt")
	if err != nil {
		t.Fatal(err)
	}
	if got := categoryNames(d.Categories()); !slices.Equal(got, []string{"break", "htt", "admin"}) {
		t.Errorf("Categories() with default htt = %v, want [break htt admin]", got)
	}
}

func categoryNames(categories []Category) []string {
	names := make([]string, len(categories))
	for i, c := range categories {
		names[i] = c.Name
	}
	return names
}

func TestCategoriser_WorkIsWhatTheCategorySays(t *testing.T) {
	// The domain takes Work as given; the CLI defaults it to true when
	// the config leaves it out (see internal/cli).
	c := mustCategoriser(t, []Category{
		{Name: "deep", Tags: []string{"#deep"}, Work: true},
		{Name: "walk", Tags: []string{"@walk"}},
	})
	if !c.Categorise(mustTask(t, "#deep")).Work {
		t.Errorf("deep should count as work")
	}
	if c.Categorise(mustTask(t, "@walk")).Work {
		t.Errorf("walk was built with Work false")
	}
}

func TestNewCategoriser_Errors(t *testing.T) {
	cases := []struct {
		name       string
		categories []Category
		want       string
	}{
		{"empty name", []Category{{Name: "", Tags: []string{"@a"}}}, "category 1: no name"},
		{"blank name", []Category{{Name: "  ", Tags: []string{"@a"}}}, "category 1: no name"},
		{"duplicate name", []Category{
			{Name: "break", Tags: []string{"@a"}},
			{Name: "htt", Tags: []string{"+htt"}},
			{Name: "break", Tags: []string{"@b"}},
		}, `category 3 (break): the name is already category 1's`},
		{"bare word", []Category{{Name: "htt", Tags: []string{"htt"}}}, `category 1 (htt): "htt" isn't a tag (want +project, @context or #tag)`},
		{"sigil alone", []Category{{Name: "htt", Tags: []string{"+"}}}, `"+" isn't a tag`},
		{"empty tag", []Category{{Name: "htt", Tags: []string{""}}}, `"" isn't a tag`},
		{"space in tag", []Category{{Name: "htt", Tags: []string{"+my project"}}}, `"+my project" isn't a tag`},
		{"tag in two categories", []Category{
			{Name: "break", Tags: []string{"@lunch"}},
			{Name: "food", Tags: []string{"@cook", "@lunch"}},
		}, "category 2 (food): @lunch is already in category 1 (break)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCategoriser(tc.categories)
			if err == nil {
				t.Fatalf("NewCategoriser: want an error containing %q, got none", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestCategoriser_Check_ACategoryWithoutTagsMustBeTheDefault(t *testing.T) {
	tagless := []Category{
		{Name: "break", Tags: []string{"@lunch"}},
		{Name: "other"},
	}
	c, err := NewCategoriser(tagless)
	if err != nil {
		t.Fatalf("NewCategoriser: %v", err)
	}
	// With working as the default, nothing can reach "other".
	want := "category 2 (other): no tags, and it isn't the default category"
	if err := c.Check(); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Check() = %v, want an error containing %q", err, want)
	}
	// As the default, it's where unmatched entries go.
	d, err := c.WithDefault("other")
	if err != nil {
		t.Fatalf("WithDefault(other): %v", err)
	}
	if err := d.Check(); err != nil {
		t.Errorf("Check() with other as the default: %v", err)
	}
	// So is a tagless working, the built-in default's name.
	w, err := NewCategoriser([]Category{{Name: "working", Work: false}})
	if err != nil {
		t.Fatalf("NewCategoriser: %v", err)
	}
	if err := w.Check(); err != nil {
		t.Errorf("Check() with a tagless working: %v", err)
	}
	var none *Categoriser
	if err := none.Check(); err != nil {
		t.Errorf("Check() on no categoriser: %v", err)
	}
}

func TestNewCategoriser_NamesAreExact(t *testing.T) {
	// Names are matched exactly, like tags: "Break" and "break" are two
	// categories.
	c := mustCategoriser(t, []Category{
		{Name: "break", Tags: []string{"@break"}},
		{Name: "Break", Tags: []string{"@Break"}},
	})
	if got := c.Categorise(mustTask(t, "@Break")).Name; got != "Break" {
		t.Errorf("Categorise(@Break) = %q, want Break", got)
	}
}

func TestCategoriser_WithDefault(t *testing.T) {
	base := mustCategoriser(t, testCategories())
	cases := []struct {
		name     string
		def      string
		wantName string
		wantWork bool
	}{
		{"the built-in working", "working", "working", true},
		{"a category that counts as work", "htt", "htt", true},
		// Its work setting decides: unmatched time then isn't worked.
		{"a category that doesn't count as work", "break", "break", false},
		{"surrounding spaces", " admin ", "admin", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := base.WithDefault(tc.def)
			if err != nil {
				t.Fatalf("WithDefault(%q): %v", tc.def, err)
			}
			got := c.Categorise(mustTask(t, "unmatched entry"))
			if got.Name != tc.wantName || got.Work != tc.wantWork {
				t.Errorf("unmatched entry -> %+v, want %s (work %v)", got, tc.wantName, tc.wantWork)
			}
			// The categories themselves are unchanged.
			if got := c.Categorise(mustTask(t, "@lunch")); got.Name != "break" {
				t.Errorf("@lunch -> %q, want break", got.Name)
			}
		})
	}
	// WithDefault returns a copy.
	if got := base.Categorise(mustTask(t, "unmatched")); got.Name != DefaultCategoryName {
		t.Errorf("WithDefault changed the categoriser it was called on: unmatched -> %q", got.Name)
	}
}

func TestCategoriser_WorkingDefinedInTheListIsTheDefault(t *testing.T) {
	// When the list defines "working", its definition applies, work
	// setting included, to the entries no category matches.
	c := mustCategoriser(t, []Category{
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
		{Name: "working", Tags: []string{"@desk"}, Work: false},
	})
	for _, entry := range []string{"unmatched", "@desk"} {
		got := c.Categorise(mustTask(t, entry))
		if got.Name != "working" || got.Work {
			t.Errorf("%q -> %+v, want the list's working (work false)", entry, got)
		}
	}
	if names := categoryNames(c.Categories()); !slices.Equal(names, []string{"htt", "working"}) {
		t.Errorf("Categories() = %v, want [htt working]", names)
	}
	d, err := c.WithDefault("working")
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Categorise(mustTask(t, "unmatched")); got.Work {
		t.Errorf("WithDefault(working) should keep the list's working, work false; got %+v", got)
	}
}

func TestCategoriser_WithDefault_UndefinedNameIsAnError(t *testing.T) {
	base := mustCategoriser(t, testCategories())
	for _, def := range []string{"misc", "Working", "Break", "", "  "} {
		_, err := base.WithDefault(def)
		if err == nil {
			t.Errorf("WithDefault(%q): want an error", def)
			continue
		}
		want := fmt.Sprintf("%q isn't a category (want one of: break, htt, admin, working)", strings.TrimSpace(def))
		if err.Error() != want {
			t.Errorf("WithDefault(%q) error = %q, want %q", def, err, want)
		}
	}
	_, err := mustCategoriser(t, nil).WithDefault("misc")
	if err == nil || err.Error() != `"misc" isn't a category (want one of: working)` {
		t.Errorf("WithDefault(misc) with no categories: error = %v", err)
	}
}

// TestCategoriser_FirstMatchProperty checks Match against the rule read
// straight off the entry's words, over generated category lists,
// defaults and entries: the result is the first category with a tag
// that is one of the entry's words, verbatim, matched; or the default
// when there is none, unmatched.
func TestCategoriser_FirstMatchProperty(t *testing.T) {
	// Tags, and near misses that must never match them.
	pool := []string{"+htt", "+admin", "@lunch", "@break", "@email", "#deep", "+home", "@phone"}
	misses := []string{"+HTT", "@Lunch", "@breakfast", "@break-time", "#Deep", "+htt.", "lunch", "break"}
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 5000 {
		// Categories: a random partition of a shuffled sample of the pool.
		tags := slices.Clone(pool)
		r.Shuffle(len(tags), func(a, b int) { tags[a], tags[b] = tags[b], tags[a] })
		tags = tags[:1+r.IntN(len(tags))]
		var categories []Category
		for len(tags) > 0 {
			n := 1 + r.IntN(min(3, len(tags)))
			categories = append(categories, Category{
				Name: fmt.Sprintf("c%d", len(categories)),
				Tags: tags[:n],
				Work: r.IntN(2) == 0,
			})
			tags = tags[n:]
		}
		c := mustCategoriser(t, categories)
		def := Category{Name: DefaultCategoryName, Work: true}
		if r.IntN(3) == 0 { // one of the categories as the default
			def = categories[r.IntN(len(categories))]
			var err error
			if c, err = c.WithDefault(def.Name); err != nil {
				t.Fatal(err)
			}
		}

		// An entry: a few words, tags and near misses in any order.
		words := []string{"doing"}
		for range r.IntN(5) {
			if r.IntN(2) == 0 {
				words = append(words, pool[r.IntN(len(pool))])
			} else {
				words = append(words, misses[r.IntN(len(misses))])
			}
		}
		r.Shuffle(len(words), func(a, b int) { words[a], words[b] = words[b], words[a] })
		text := strings.Join(words, " ")

		want, wantMatched := def, false
	search:
		for _, cat := range categories {
			for _, tag := range cat.Tags {
				if slices.Contains(words, tag) {
					want, wantMatched = cat, true
					break search
				}
			}
		}
		got, matched := c.Match(mustTask(t, text))
		if got.Name != want.Name || got.Work != want.Work || matched != wantMatched {
			t.Fatalf("case %d: categories %+v, default %s\nMatch(%q) = %+v, matched %v; want %+v, matched %v",
				i, categories, def.Name, text, got, matched, want, wantMatched)
		}
	}
}
