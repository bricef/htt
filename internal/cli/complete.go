package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bricef/htt/internal/domain"
	"github.com/spf13/cobra"
)

var doLeaveSubtasks, doWithSubtasks bool

var do = &cobra.Command{
	Use:     "do <item index>",
	Short:   "Complete a task",
	Aliases: []string{"x"},
	Long: `Complete a task: it moves to the done file, marked done today.

A task with open subtasks asks first, on the terminal:

  Task 3 has 2 open subtasks.
  [x] Mark it done, and leave the subtasks as they are.
  [A] Mark the subtasks done too.
  Anything else cancels.

Answer with x or A, exactly as written (case matters: x is lower case,
A a capital), then Enter.
x completes the task alone: its subtasks stay open, still naming it
with p:. A completes it and every open task below it, at any depth.
Anything else, or the end of the input, changes nothing.

Without a terminal to ask on (a script, piped input), give the answer
as a flag: --leave-subtasks (as x) or --with-subtasks (as A). Without
one, the task isn't completed, and htt says why.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if doLeaveSubtasks && doWithSubtasks {
			return errors.New("--leave-subtasks and --with-subtasks exclude each other: give one")
		}
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		choice := domain.SubtasksAsk
		switch {
		case doLeaveSubtasks:
			choice = domain.SubtasksLeave
		case doWithSubtasks:
			choice = domain.SubtasksComplete
		default:
			open, err := ctx.OpenSubtasks(args[0])
			if err != nil {
				return fmt.Errorf("complete task: %w", err)
			}
			if len(open) > 0 {
				answer, err := askAboutSubtasks(args[0], len(open))
				if err != nil {
					return err
				}
				if answer == domain.SubtasksAsk {
					fmt.Println("Cancelled: nothing changed.")
					return nil
				}
				choice = answer
			}
		}
		done, err := ctx.Complete(args[0], choice)
		if err != nil {
			return fmt.Errorf("complete task: %w", err)
		}
		for _, t := range done {
			fmt.Printf("Completed: %s\n", t.ConsoleString())
		}
		return nil
	},
}

func init() {
	do.Flags().BoolVar(&doLeaveSubtasks, "leave-subtasks", false,
		"If the task has open subtasks, complete it alone, leaving them open (the prompt's x)")
	do.Flags().BoolVar(&doWithSubtasks, "with-subtasks", false,
		"If the task has open subtasks, complete them too, at any depth (the prompt's A)")
}

// askAboutSubtasks asks, on the terminal, what to do with task n's open
// subtasks, and returns the answer: SubtasksLeave for x, SubtasksComplete
// for A, SubtasksAsk (cancel) for anything else or the end of the input.
// Without a terminal (stdin and stdout both one) it asks nothing and
// fails, naming the flags that answer it.
//
// The question runs through runOnTerminal, as the TUI does: in the REPL
// that closes readline while the answer is read, so it can't take the
// answer for a command line.
func askAboutSubtasks(n string, open int) (domain.Subtasks, error) {
	if !onTerminal() {
		return domain.SubtasksAsk, fmt.Errorf("task %s has %s, and there's no terminal to ask on: "+
			"give --leave-subtasks to complete it alone, or --with-subtasks to complete them too", n, openSubtasks(open))
	}
	var answer string
	err := runOnTerminal(func() error {
		fmt.Print(strings.Join(domain.SubtasksQuestion(n, open), "\n") + "\n")
		line, err := readAnswer(os.Stdin)
		if err == nil {
			answer = line
		}
		return nil // the end of the input, or a failure to read it, cancels
	})
	if err != nil {
		return domain.SubtasksAsk, err
	}
	switch answer {
	case "x":
		return domain.SubtasksLeave, nil
	case "A":
		return domain.SubtasksComplete, nil
	}
	return domain.SubtasksAsk, nil
}

// openSubtasks says how many open subtasks there are: "1 open subtask",
// "2 open subtasks".
func openSubtasks(n int) string {
	if n == 1 {
		return "1 open subtask"
	}
	return fmt.Sprintf("%d open subtasks", n)
}

// readAnswer reads one line from r, a byte at a time so nothing after it
// is taken from whatever reads next, and returns it without its line
// ending (\n or \r\n). The end of the input before a line ends is
// io.EOF.
func readAnswer(r io.Reader) (string, error) {
	var line []byte
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n == 1 {
			if b[0] == '\n' {
				return strings.TrimSuffix(string(line), "\r"), nil
			}
			line = append(line, b[0])
			continue
		}
		if err != nil {
			return "", err
		}
	}
}
