package cli

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/vars"
	"github.com/spf13/viper"
)

// categoriserFromConfig reads timelog.categories and
// timelog.default_category from v. A config that isn't valid is an
// error naming the key and what is wrong with it. The domain holds no
// config: the CLI reads it and hands the domain a Categoriser, as
// `htt interactive` builds the TUI's theme.
//
//	timelog:
//	  categories:            # first match wins
//	    - name: break
//	      tags: ["@break", "@lunch"]
//	      work: false        # optional; categories count as work
//	    - name: htt
//	      tags: ["+htt"]
//	  default_category: working
func categoriserFromConfig(v *viper.Viper) (*domain.Categoriser, error) {
	switch section := v.Get(vars.ConfigKeyTimelog).(type) {
	case nil:
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(section)) {
			if key != "categories" && key != "default_category" {
				return nil, fmt.Errorf("config: %s: unknown key %q (want categories or default_category)", vars.ConfigKeyTimelog, key)
			}
		}
	default:
		return nil, fmt.Errorf("config: %s: want categories and default_category, got %q", vars.ConfigKeyTimelog, fmt.Sprint(section))
	}

	list, err := parseCategories(v.Get(vars.ConfigKeyTimelogCategories))
	if err != nil {
		return nil, fmt.Errorf("config: %s: %w", vars.ConfigKeyTimelogCategories, err)
	}
	c, err := domain.NewCategoriser(list)
	if err != nil {
		return nil, fmt.Errorf("config: %s: %w", vars.ConfigKeyTimelogCategories, err)
	}
	def := domain.DefaultCategoryName
	if v.IsSet(vars.ConfigKeyTimelogDefaultCategory) {
		def = v.GetString(vars.ConfigKeyTimelogDefaultCategory)
	}
	c, err = c.WithDefault(def)
	if err != nil {
		return nil, fmt.Errorf("config: %s: %w", vars.ConfigKeyTimelogDefaultCategory, err)
	}
	return c, nil
}

// parseCategories reads the categories list as the YAML decoder gives
// it. It checks the shape (a list of maps with known keys and values
// of the right type); domain.NewCategoriser checks the rest.
func parseCategories(raw any) ([]domain.Category, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		if _, isMap := stringMap(raw); isMap {
			return nil, fmt.Errorf("want a list (- name: …), one item per category, in order: the first category with a matching tag wins")
		}
		return nil, fmt.Errorf("want a list (- name: …), one item per category, got %q", fmt.Sprint(raw))
	}
	list := make([]domain.Category, 0, len(items))
	for i, item := range items {
		cat, err := parseCategory(item)
		if err != nil {
			label := fmt.Sprintf("category %d", i+1)
			if name := strings.TrimSpace(cat.Name); name != "" {
				label += " (" + name + ")"
			}
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		list = append(list, cat)
	}
	return list, nil
}

// parseCategory reads one category. On an error, the category returned
// carries the name, if it read one, to label the error with.
func parseCategory(item any) (domain.Category, error) {
	fields, ok := stringMap(item)
	if !ok {
		return domain.Category{}, fmt.Errorf("want name, tags and work, got %q", fmt.Sprint(item))
	}
	cat := domain.Category{Work: true} // categories count as work unless they say otherwise
	if value, ok := fields["name"]; ok && value != nil {
		name, ok := value.(string)
		if !ok {
			return cat, fmt.Errorf("name: want text, got %v", value)
		}
		cat.Name = name
	}
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		value := fields[key]
		switch key {
		case "name":
		case "tags":
			tags, err := parseTags(value)
			if err != nil {
				return cat, fmt.Errorf("tags: %w", err)
			}
			cat.Tags = tags
		case "work":
			work, ok := value.(bool)
			if !ok {
				if value == nil {
					return cat, fmt.Errorf("work: want true or false, got nothing")
				}
				return cat, fmt.Errorf("work: want true or false, got %q", fmt.Sprint(value))
			}
			cat.Work = work
		default:
			return cat, fmt.Errorf("unknown key %q (want name, tags or work)", key)
		}
	}
	return cat, nil
}

// parseTags reads a category's tags: a list of text.
func parseTags(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf(`want a list, such as ["@lunch"], got %q`, fmt.Sprint(raw))
	}
	tags := make([]string, 0, len(items))
	for i, item := range items {
		switch tag := item.(type) {
		case string:
			tags = append(tags, tag)
		case nil:
			// Unquoted, YAML reads "- #deep" as a comment: no value.
			return nil, fmt.Errorf(`tag %d is empty; quote tags in YAML ("#deep"), since an unquoted # starts a comment`, i+1)
		default:
			return nil, fmt.Errorf("tag %d: want text, got %v", i+1, item)
		}
	}
	return tags, nil
}

// stringMap returns a YAML mapping as a map with lower-case string
// keys, as viper treats keys. The YAML decoder gives a mapping inside a
// list as map[any]any; viper turns only the mappings outside lists
// into map[string]any.
func stringMap(raw any) (map[string]any, bool) {
	out := map[string]any{}
	switch m := raw.(type) {
	case map[string]any:
		for k, v := range m {
			out[strings.ToLower(k)] = v
		}
	case map[any]any:
		for k, v := range m {
			out[strings.ToLower(fmt.Sprint(k))] = v
		}
	default:
		return nil, false
	}
	return out, true
}
