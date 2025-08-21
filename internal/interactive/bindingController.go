package interactive

import (
	"slices"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func filter[T any](s []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(s)) // Pre-allocate for efficiency
	for _, v := range s {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

type Binding struct {
	action  *Action
	binding key.Binding
	short   bool
}

type KeyBindingController struct {
	bindings []Binding
}

func NewKeyBindingController() KeyBindingController {
	return KeyBindingController{
		bindings: []Binding{},
	}
}

func (c KeyBindingController) AddShortBinding(action *Action, binding key.Binding) KeyBindingController {
	c.bindings = append(c.bindings, Binding{action, binding, true})
	return c
}

func (c KeyBindingController) AddBinding(action *Action, binding key.Binding) KeyBindingController {
	c.bindings = append(c.bindings, Binding{action, binding, false})
	return c
}

func (c KeyBindingController) ClearBinding(action *Action) KeyBindingController {
	c.bindings = filter(c.bindings, func(b Binding) bool { return b.action != action })
	return c
}

func (k KeyBindingController) ShortHelp() []key.Binding {
	short := []key.Binding{}
	for _, binding := range k.bindings {
		if binding.short {
			short = append(short, binding.binding)
		}
	}
	return short
}

func (k KeyBindingController) FullHelp() [][]key.Binding {

	// 4 columns
	// last column is help and quit

	len := len(k.bindings)
	perCol := len / 4

	chunks := slices.Chunk(k.bindings, perCol)

	help := [][]key.Binding{}

	for chunk := range chunks {
		row := []key.Binding{}
		for _, binding := range chunk {
			row = append(row, binding.binding)
		}
		help = append(help, row)
	}

	// help = append(help, []key.Binding{k[Help], k[Quit]})

	return help
}

func (k KeyBindingController) GetAction(msg tea.KeyMsg) *Action {
	for _, binding := range k.bindings {
		if key.Matches(msg, binding.binding) {
			return binding.action
		}
	}
	return Noop
}
