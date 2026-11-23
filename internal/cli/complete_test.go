package cli

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
)

// The question takes the terminal as the TUI does (runOnTerminal, the
// REPL's hand-over), reads its answer there, and takes x and A as they
// are written, anything else as cancelling.
func TestAskAboutSubtasks(t *testing.T) {
	prevTerminal, prevRun, prevStdin, prevStdout := onTerminal, runOnTerminal, os.Stdin, os.Stdout
	t.Cleanup(func() { onTerminal, runOnTerminal, os.Stdin, os.Stdout = prevTerminal, prevRun, prevStdin, prevStdout })
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = devNull.Close() }()
	os.Stdout = devNull

	for _, tc := range []struct {
		input string
		want  domain.Subtasks
	}{
		{"x\n", domain.SubtasksLeave},
		{"A\n", domain.SubtasksComplete},
		{"a\n", domain.SubtasksAsk},
		{"", domain.SubtasksAsk},
	} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.WriteString(tc.input)
		_ = w.Close()
		os.Stdin = r
		onTerminal = func() bool { return true }
		handedOver := false
		runOnTerminal = func(run func() error) error {
			handedOver = true
			return run()
		}
		got, err := askAboutSubtasks("3", 2)
		_ = r.Close()
		if err != nil || got != tc.want {
			t.Errorf("answer %q: %v, %v; want %v", tc.input, got, err, tc.want)
		}
		if !handedOver {
			t.Errorf("answer %q: read without runOnTerminal", tc.input)
		}
	}

	onTerminal = func() bool { return false }
	if _, err := askAboutSubtasks("3", 2); err == nil || !strings.Contains(err.Error(), "--leave-subtasks") {
		t.Errorf("without a terminal: %v", err)
	}
}

// readAnswer takes one line and no more, so what follows is left for
// whatever reads next (the REPL's next command).
func TestReadAnswer(t *testing.T) {
	cases := []struct {
		input, want, rest string
		err               error
	}{
		{"x\n", "x", "", nil},
		{"A\r\n", "A", "", nil},
		{"x\nadd milk\n", "x", "add milk\n", nil},
		{" x \n", " x ", "", nil},
		{"\n", "", "", nil},
		{"x", "", "", io.EOF}, // the end of the input before the line ends
		{"", "", "", io.EOF},
	}
	for _, tc := range cases {
		r := strings.NewReader(tc.input)
		got, err := readAnswer(r)
		if got != tc.want || !errors.Is(err, tc.err) {
			t.Errorf("readAnswer(%q) = %q, %v; want %q, %v", tc.input, got, err, tc.want, tc.err)
		}
		if rest, _ := io.ReadAll(r); string(rest) != tc.rest {
			t.Errorf("readAnswer(%q) left %q, want %q", tc.input, rest, tc.rest)
		}
	}
}
