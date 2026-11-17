package domain

import (
	"fmt"
	"regexp"
	"strconv"
)

// isoDate matches a word written the way todo.txt writes a date,
// YYYY-MM-DD, whether or not it is a real date.
var isoDate = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

// looseDate matches a word plainly meant as a date but not written
// YYYY-MM-DD: a month or day without its leading zero (2026-5-3).
var looseDate = regexp.MustCompile(`^([0-9]{4})-([0-9]{1,2})-([0-9]{1,2})$`)

// DateWarnings lists, in line order, the dates on the task's line that
// htt can't read as dates, each as a message naming the word:
//
//   - a word that looks like a date but isn't one: an impossible date
//     (2026-13-45, 2026-02-30) or one that isn't written YYYY-MM-DD
//     (2026-5-3). It is part of the task's text, like any other word.
//   - the due: value Task.Due reads (the last due: on the line) when
//     it isn't a YYYY-MM-DD date. The task then has no due date, so
//     due-date filters and sorting pass it over.
//
// A line with these still loads and keeps every word: the warnings
// only say how it was read, for the CLI (stderr) and the TUI (the
// status line and the task input) to show. Preamble dates are always
// real (the parser only reads a real date there), and no other
// annotation is checked: htt reads no dates from them.
func (t *Task) DateWarnings() []string {
	lastDue := -1
	for i, w := range t.body {
		if w.key == "due" {
			lastDue = i
		}
	}
	var out []string
	for i, w := range t.body {
		switch {
		case w.key == "":
			if problem := dateProblem(w.text); problem != "" {
				out = append(out, fmt.Sprintf("%s %s, so it's text", w.text, problem))
			}
		case i == lastDue:
			value := t.Annotations["due"]
			if _, ok := ParseLocalDate(value); ok {
				continue
			}
			problem := dateProblem(value)
			if problem == "" {
				problem = "isn't a YYYY-MM-DD date"
			}
			out = append(out, fmt.Sprintf("due:%s %s, so the task has no due date", value, problem))
		}
	}
	return out
}

// dateProblem says why a word that looks like a date isn't one htt
// reads: "isn't a real date", or "isn't written YYYY-MM-DD
// (2026-05-03)" with the date it would be. It returns "" for a real
// YYYY-MM-DD date and for a word that doesn't look like a date.
func dateProblem(word string) string {
	if isoDate.MatchString(word) {
		if _, ok := ParseLocalDate(word); ok {
			return ""
		}
		return "isn't a real date"
	}
	m := looseDate.FindStringSubmatch(word)
	if m == nil {
		return ""
	}
	month, _ := strconv.Atoi(m[2]) // the pattern makes these numbers
	day, _ := strconv.Atoi(m[3])
	padded := fmt.Sprintf("%s-%02d-%02d", m[1], month, day)
	if _, ok := ParseLocalDate(padded); !ok {
		return "isn't a real date"
	}
	return fmt.Sprintf("isn't written YYYY-MM-DD (%s)", padded)
}
