package cli

import (
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/vars"
	"github.com/spf13/viper"
)

// configFrom reads yaml as htt reads its config file, with the default
// main.go sets for timelog.default_category.
func configFrom(t *testing.T, yaml string) *viper.Viper {
	t.Helper()
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}
	v.SetDefault(vars.ConfigKeyTimelogDefaultCategory, vars.DefaultTimelogCategory)
	return v
}

// categoryOf categorises an entry with the given text.
func categoryOf(t *testing.T, c *domain.Categoriser, text string) domain.Category {
	t.Helper()
	entry, err := domain.NewTask(text)
	if err != nil {
		t.Fatal(err)
	}
	return c.Categorise(entry)
}

const planConfig = `
timelog:
  categories:            # first match wins
    - name: break
      tags: ["@break", "@lunch"]
      work: false
    - name: htt
      tags: ["+htt"]
    - name: admin
      tags: ["@email", "+admin"]
  default_category: working
`

func TestTimelogConfig_ThePlansExample(t *testing.T) {
	c, err := categoriserFromConfig(configFrom(t, planConfig))
	if err != nil {
		t.Fatalf("categoriserFromConfig: %v", err)
	}
	for text, want := range map[string]domain.Category{
		"@lunch":           {Name: "break", Work: false},
		"@break coffee":    {Name: "break", Work: false},
		"work on it +htt":  {Name: "htt", Work: true}, // work defaults to true
		"mail @email":      {Name: "admin", Work: true},
		"+htt @lunch":      {Name: "break", Work: false},
		"@breakfast":       {Name: "working", Work: true},
		"@Break":           {Name: "working", Work: true},
		"reading +reading": {Name: "working", Work: true},
	} {
		if got := categoryOf(t, c, text); got.Name != want.Name || got.Work != want.Work {
			t.Errorf("%q -> %s (work %v), want %s (work %v)", text, got.Name, got.Work, want.Name, want.Work)
		}
	}
}

func TestTimelogConfig_TheDefaultsAgree(t *testing.T) {
	// main.go writes vars' default into a fresh config; the domain
	// falls back on its own.
	if vars.DefaultTimelogCategory != domain.DefaultCategoryName {
		t.Errorf("vars.DefaultTimelogCategory = %q, domain.DefaultCategoryName = %q", vars.DefaultTimelogCategory, domain.DefaultCategoryName)
	}
}

func TestTimelogConfig_NoConfigIsWorking(t *testing.T) {
	for _, yaml := range []string{"", "timelog:\n", "timelog:\n  categories: []\n", "tui:\n  theme: nord\n"} {
		for _, withDefault := range []bool{true, false} {
			v := configFrom(t, yaml)
			if !withDefault {
				v = viper.New() // no default set: the domain's applies
				v.SetConfigType("yaml")
				_ = v.ReadConfig(strings.NewReader(yaml))
			}
			c, err := categoriserFromConfig(v)
			if err != nil {
				t.Fatalf("config %q: %v", yaml, err)
			}
			if got := categoryOf(t, c, "@lunch +htt"); got.Name != "working" || !got.Work {
				t.Errorf("config %q: @lunch +htt -> %+v, want working", yaml, got)
			}
		}
	}
}

func TestTimelogConfig_DefaultCategory(t *testing.T) {
	categories := `
timelog:
  categories:
    - name: break
      tags: ["@break"]
      work: false
    - name: htt
      tags: ["+htt"]
`
	cases := []struct {
		name     string
		def      string // "" leaves default_category out
		wantName string
		wantWork bool
	}{
		{"unset is working", "", "working", true},
		{"working", "default_category: working", "working", true},
		{"a defined category that counts as work", "default_category: htt", "htt", true},
		// Its work setting decides: unmatched time isn't worked.
		{"a defined category that doesn't", "default_category: break", "break", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := categoriserFromConfig(configFrom(t, categories+"  "+tc.def+"\n"))
			if err != nil {
				t.Fatalf("categoriserFromConfig: %v", err)
			}
			if got := categoryOf(t, c, "unmatched"); got.Name != tc.wantName || got.Work != tc.wantWork {
				t.Errorf("unmatched -> %s (work %v), want %s (work %v)", got.Name, got.Work, tc.wantName, tc.wantWork)
			}
		})
	}
}

func TestTimelogConfig_WorkingDefinedInTheListApplies(t *testing.T) {
	for _, def := range []string{"", "  default_category: working\n"} {
		c, err := categoriserFromConfig(configFrom(t, `
timelog:
  categories:
    - name: working
      tags: ["@desk"]
      work: false
`+def))
		if err != nil {
			t.Fatalf("categoriserFromConfig: %v", err)
		}
		if got := categoryOf(t, c, "unmatched"); got.Name != "working" || got.Work {
			t.Errorf("unmatched -> %+v, want the list's working, work false", got)
		}
	}
}

func TestTimelogConfig_Errors(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{"categories as a map", "timelog:\n  categories:\n    break: [\"@lunch\"]\n",
			"config: timelog.categories: want a list (- name: …), one item per category, in order: the first category with a matching tag wins"},
		{"categories as text", "timelog:\n  categories: break\n",
			`config: timelog.categories: want a list (- name: …), one item per category, got "break"`},
		{"a category that isn't a map", "timelog:\n  categories:\n    - break\n",
			`config: timelog.categories: category 1: want name, tags and work, got "break"`},
		{"unknown key", "timelog:\n  categories:\n    - name: break\n      tags: [\"@lunch\"]\n      colour: red\n",
			`config: timelog.categories: category 1 (break): unknown key "colour" (want name, tags or work)`},
		{"unknown key in the section", "timelog:\n  categoreis: []\n",
			`config: timelog: unknown key "categoreis" (want categories or default_category)`},
		{"section isn't a map", "timelog: yes\n",
			`config: timelog: want categories and default_category, got "true"`},
		{"no name", "timelog:\n  categories:\n    - tags: [\"@lunch\"]\n",
			"config: timelog.categories: category 1: no name"},
		{"empty name", "timelog:\n  categories:\n    - name: \"\"\n      tags: [\"@lunch\"]\n",
			"config: timelog.categories: category 1: no name"},
		{"name isn't text", "timelog:\n  categories:\n    - name: [a]\n      tags: [\"@lunch\"]\n",
			"config: timelog.categories: category 1: name: want text, got [a]"},
		{"duplicate names", "timelog:\n  categories:\n    - name: break\n      tags: [\"@lunch\"]\n    - name: break\n      tags: [\"@nap\"]\n",
			"config: timelog.categories: category 2 (break): the name is already category 1's"},
		{"no tags", "timelog:\n  categories:\n    - name: break\n",
			"config: timelog.categories: category 1 (break): no tags; list the tags that put an entry in it"},
		{"empty tags", "timelog:\n  categories:\n    - name: break\n      tags: []\n",
			"config: timelog.categories: category 1 (break): no tags"},
		{"tags as text", "timelog:\n  categories:\n    - name: break\n      tags: \"@lunch\"\n",
			`config: timelog.categories: category 1 (break): tags: want a list, such as ["@lunch"], got "@lunch"`},
		// Unquoted, YAML reads #deep as a comment and the item is empty.
		{"unquoted hash tag", "timelog:\n  categories:\n    - name: deep\n      tags:\n        - #deep\n",
			`config: timelog.categories: category 1 (deep): tags: tag 1 is empty; quote tags in YAML ("#deep"), since an unquoted # starts a comment`},
		{"a bare word", "timelog:\n  categories:\n    - name: break\n      tags: [lunch]\n",
			`config: timelog.categories: category 1 (break): "lunch" isn't a tag (want +project, @context or #tag)`},
		{"a tag in two categories", "timelog:\n  categories:\n    - name: break\n      tags: [\"@lunch\"]\n    - name: food\n      tags: [\"@lunch\"]\n",
			"config: timelog.categories: category 2 (food): @lunch is already in category 1 (break)"},
		{"work isn't true or false", "timelog:\n  categories:\n    - name: break\n      tags: [\"@lunch\"]\n      work: sometimes\n",
			`config: timelog.categories: category 1 (break): work: want true or false, got "sometimes"`},
		{"work without a value", "timelog:\n  categories:\n    - name: break\n      tags: [\"@lunch\"]\n      work:\n",
			"config: timelog.categories: category 1 (break): work: want true or false, got nothing"},
		{"an undefined default", planConfig + "  default_category: misc\n",
			`config: timelog.default_category: "misc" isn't a category (want one of: break, htt, admin, working)`},
		{"default names are exact", planConfig + "  default_category: Break\n",
			`config: timelog.default_category: "Break" isn't a category`},
		{"an undefined default without categories", "timelog:\n  default_category: misc\n",
			`config: timelog.default_category: "misc" isn't a category (want one of: working)`},
		{"an empty default", "timelog:\n  default_category: \"\"\n",
			`config: timelog.default_category: "" isn't a category (want one of: working)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := categoriserFromConfig(configFrom(t, tc.config))
			if err == nil {
				t.Fatalf("want an error containing %q, got none", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q\nwant it to contain %q", err, tc.want)
			}
		})
	}
}
