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
	stack   [][]Binding
	current *[]Binding
}

func NewKeyBindingController() KeyBindingController {
	stack := make([][]Binding, 1)
	stack[0] = []Binding{}

	return KeyBindingController{
		stack:   stack,
		current: &stack[len(stack)-1],
	}
}

func (c KeyBindingController) AddShortBinding(action *Action, binding key.Binding) KeyBindingController {
	*c.current = append(*c.current, Binding{action, binding, true})
	return c
}

func (c KeyBindingController) AddBinding(action *Action, binding key.Binding) KeyBindingController {
	c.stack = append(c.stack, []Binding{{action, binding, false}})
	return c
}

func (k KeyBindingController) ShortHelp() []key.Binding {
	short := []key.Binding{}
	active := k.GetActiveBindings()
	for _, binding := range active {
		if binding.short {
			short = append(short, binding.binding)
		}
	}
	return short
}

func (k KeyBindingController) Push() {
	new_bindings := []Binding{}
	k.stack = append(k.stack, new_bindings)
	k.current = &new_bindings

}

func (k KeyBindingController) Pop() {
	k.stack = k.stack[:len(k.stack)-1]
	k.current = &k.stack[len(k.stack)-1]
}

func (k KeyBindingController) GetActiveBindings() []Binding {
	collected := []Binding{}
	for _, bindings := range k.stack {
		collected = append(collected, bindings...)
	}
	return collected
}

func (k KeyBindingController) FullHelp() [][]key.Binding {

	// Collect all active bindings
	collected := k.GetActiveBindings()

	// 4 columns
	// last column is help and quit

	len := len(collected)
	// round up
	perCol := len / 4

	chunks := slices.Chunk(collected, perCol+1)

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
	for _, binding := range k.GetActiveBindings() {
		if key.Matches(msg, binding.binding) {
			return binding.action
		}
	}
	return Noop
}
