package domain

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// DefaultCategoryName is the category of the time-log entries no
// category matches, unless a Categoriser is given another default
// (WithDefault). It counts as work.
const DefaultCategoryName = "working"

// Category classifies time-log entries by their todo.txt tags. Work
// says whether its time counts as worked time; a break doesn't.
type Category struct {
	Name string
	Tags []string // "+project", "@context", "#tag"; see Categoriser.Categorise
	Work bool
}

// Categoriser puts each time-log entry in a category: the first of its
// categories with one of the entry's tags, or else its default
// category. It holds no config: the CLI reads the categories from the
// config file and builds one with NewCategoriser and WithDefault.
//
// A nil *Categoriser has no categories, and puts every entry in the
// built-in default, working.
type Categoriser struct {
	categories []Category
	fallback   Category // the default category
}

// builtinDefault is the default category when none is chosen: working,
// which counts as work.
func builtinDefault() Category {
	return Category{Name: DefaultCategoryName, Work: true}
}

// NewCategoriser checks categories and returns a Categoriser for them,
// in that order. Each category needs a name no other has (names, like
// tags, are compared exactly) and at least one tag. A tag is a todo.txt
// tag, sigil included (+project, @context, #tag), and can be in one
// category only: an entry goes to the first category with a matching
// tag, so a later category could never get it. Errors name a category
// by its position in the list, counting from 1.
//
// The default category is the list's "working" when it defines one,
// whose Work then applies, and otherwise the built-in working, which
// counts as work; WithDefault picks another.
func NewCategoriser(categories []Category) (*Categoriser, error) {
	c := &Categoriser{fallback: builtinDefault()}
	names := map[string]int{} // name -> position
	owners := map[string]int{}
	for i, cat := range categories {
		pos := i + 1
		name := strings.TrimSpace(cat.Name)
		if name == "" {
			return nil, fmt.Errorf("category %d: no name", pos)
		}
		label := fmt.Sprintf("category %d (%s)", pos, name)
		if other, ok := names[name]; ok {
			return nil, fmt.Errorf("%s: the name is already category %d's", label, other)
		}
		names[name] = pos
		if len(cat.Tags) == 0 {
			return nil, fmt.Errorf("%s: no tags; list the tags that put an entry in it", label)
		}
		var tags []string
		for _, tag := range cat.Tags {
			if !isTag(tag) {
				return nil, fmt.Errorf("%s: %q isn't a tag (want +project, @context or #tag)", label, tag)
			}
			if other, ok := owners[tag]; ok {
				if other == pos {
					continue // listed twice in the same category: harmless
				}
				return nil, fmt.Errorf("%s: %s is already in category %d (%s), and an entry goes to the first category with a matching tag",
					label, tag, other, c.categories[other-1].Name)
			}
			owners[tag] = pos
			tags = append(tags, tag)
		}
		cat = Category{Name: name, Tags: tags, Work: cat.Work}
		c.categories = append(c.categories, cat)
		if name == DefaultCategoryName {
			c.fallback = cat
		}
	}
	return c, nil
}

// isTag reports whether s is a todo.txt tag: a sigil (+, @ or #) and at
// least one more character, with no space anywhere, as the parser
// reads a tag in a task.
func isTag(s string) bool {
	if len(s) < 2 || !strings.ContainsRune("+@#", rune(s[0])) {
		return false
	}
	return !strings.ContainsFunc(s, unicode.IsSpace)
}

// WithDefault returns a copy of c whose default category, for the
// entries no category matches, is the one named name: one of c's
// categories, whose Work then decides whether that time counts as work,
// or working, the built-in default (or the list's working, if it
// defines one). Any other name is an error.
func (c *Categoriser) WithDefault(name string) (*Categoriser, error) {
	name = strings.TrimSpace(name)
	d := &Categoriser{fallback: builtinDefault()}
	if c != nil {
		d.categories = c.categories
	}
	for _, cat := range d.categories {
		if cat.Name == name {
			d.fallback = cat
			return d, nil
		}
	}
	if name == DefaultCategoryName {
		return d, nil
	}
	return nil, fmt.Errorf("%q isn't a category (want one of: %s)", name,
		strings.Join(categoryNamesOf(d.withBuiltin()), ", "))
}

// Categorise returns entry's category: the first of c's categories
// with a tag the entry carries, or the default category. Tags match
// exactly: a category's @break matches an entry's @break, and not
// @Break, @breakfast or @break-time.
func (c *Categoriser) Categorise(entry *Task) Category {
	if c == nil {
		return builtinDefault()
	}
	for _, cat := range c.categories {
		for _, tag := range cat.Tags {
			if carriesTag(entry, tag) {
				return cat
			}
		}
	}
	return c.fallback
}

// carriesTag reports whether entry has the tag, exactly as written.
// Unlike Task.HasTag, a bare name matches nothing.
func carriesTag(entry *Task, tag string) bool {
	for _, values := range entry.Tags {
		if slices.Contains(values, tag) {
			return true
		}
	}
	return false
}

// Categories lists the categories an entry can get, in the order to
// show them: c's own, then the default category if it isn't one of
// them.
func (c *Categoriser) Categories() []Category {
	if c == nil {
		return []Category{builtinDefault()}
	}
	return c.withBuiltin()
}

// withBuiltin is c's categories, then its default if that isn't one of
// them.
func (c *Categoriser) withBuiltin() []Category {
	list := slices.Clone(c.categories)
	if !slices.ContainsFunc(list, func(cat Category) bool { return cat.Name == c.fallback.Name }) {
		list = append(list, c.fallback)
	}
	return list
}

func categoryNamesOf(categories []Category) []string {
	names := make([]string, len(categories))
	for i, cat := range categories {
		names[i] = cat.Name
	}
	return names
}
