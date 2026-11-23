package storage

import (
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// Subtasks: htt keeps every subtree below its parent in the file
// (docs/design/subtasks.md, "Order in the file"). Reordering must never
// drop, duplicate or change a line, and a file already in order is
// written back as it was, byte for byte.

// orderWords are words a line can hold, the awkward ones included.
var orderWords = []string{
	"buy", "milk", "x", "(B)", "café", "日本語", "+proj", "@home", "#tag", "due:2026-05-20",
	"2026-13-45", "10:30", "https://example.com/x", "a:b:c", "k:", "+", "id:", "p:",
	"due:friday", "2026-5-3", "context:work",
}

func randomOrderID(r *rand.Rand) string {
	switch r.IntN(12) {
	case 0:
		return "abc"
	case 1:
		return "01"
	default:
		return strconv.Itoa(1 + r.IntN(9))
	}
}

// randomOrderLine is a line as a person or another tool leaves it: a
// preamble htt rewrites (done mark, priority, dates) or none, odd
// words, odd spacing, and id: and p: tags anywhere, none, one or
// several of each.
func randomOrderLine(r *rand.Rand) string {
	var pre []string
	if r.IntN(5) == 0 {
		pre = append(pre, "x")
	}
	if r.IntN(2) == 0 {
		pre = append(pre, []string{"(A)", "(B)", "(C)", "(Q)"}[r.IntN(4)])
	}
	if r.IntN(3) == 0 {
		pre = append(pre, "2026-02-1"+strconv.Itoa(r.IntN(10)))
	}
	body := []string{"task" + strconv.Itoa(r.IntN(50))}
	for range r.IntN(4) {
		body = append(body, orderWords[r.IntN(len(orderWords))])
	}
	insert := func(word string) {
		body = slices.Insert(body, r.IntN(len(body)+1), word)
	}
	for range []int{0, 0, 0, 1, 1, 1, 1, 1, 2}[r.IntN(9)] {
		insert("id:" + randomOrderID(r))
	}
	for range []int{0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 3}[r.IntN(11)] {
		insert("p:" + randomOrderID(r))
	}
	sep := []string{" ", " ", " ", "  ", "\t"}[r.IntN(5)]
	return strings.Join(append(pre, body...), sep)
}

// randomOrderFile is a context file as htt reads it: one task a line,
// trimmed, no blank lines.
func randomOrderFile(r *rand.Rand) []string {
	lines := make([]string, r.IntN(25))
	for i := range lines {
		lines[i] = strings.TrimSpace(randomOrderLine(r))
	}
	return lines
}

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// sameLines reports whether a and b hold the same lines, each as often.
func sameLines(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func TestFileRepository_OrderKeepsEveryLine(t *testing.T) {
	r := rand.New(rand.NewPCG(31, 4))
	for range 300 {
		dir := t.TempDir()
		repo := NewFileRepository(dir, dir)
		path := repo.ContextPath("work")
		lines := randomOrderFile(r)
		writeLines(t, path, lines)

		// Loading puts the tasks in tree order; saving writes them so.
		ctx, err := repo.Context("work")
		if err != nil {
			t.Fatalf("load %q: %v", lines, err)
		}
		if err := repo.Save(ctx); err != nil {
			t.Fatal(err)
		}
		once := []string{}
		if len(lines) > 0 {
			once = readLines(t, path)
		}
		if !sameLines(once, lines) {
			t.Fatalf("reordering changed the lines:\n  %q\nbecame\n  %q", lines, once)
		}

		// Now in order: loading and saving again changes nothing, byte
		// for byte, and the numbers are the lines.
		before, _ := os.ReadFile(path)
		again, err := repo.Context("work")
		if err != nil {
			t.Fatal(err)
		}
		for i, task := range again.Tasks {
			if task.Line != i || task.FileLine != i+1 {
				t.Fatalf("task %q: Line %d, FileLine %d in a file in order, want %d and %d", task.Raw, task.Line, task.FileLine, i, i+1)
			}
		}
		if err := repo.Save(again); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatalf("a file in order changed on writing:\n  %q\nbecame\n  %q", before, after)
		}

		// Adding a task to a file in order changes nothing above it.
		if err := again.AddTask(mustTask(t, "2026-02-01 one more")); err != nil {
			t.Fatal(err)
		}
		added, _ := os.ReadFile(path)
		if string(added) != string(before)+"2026-02-01 one more\n" {
			t.Fatalf("adding a task to\n  %q\nwrote\n  %q", before, added)
		}
	}
}

// A file htt wrote in order before subtasks, or one with subtasks
// already in order, comes back unchanged whatever the command does
// elsewhere in it.
func TestFileRepository_AFileInOrderIsWrittenBackUnchanged(t *testing.T) {
	dir := t.TempDir()
	repo := NewFileRepository(dir, dir)
	lines := []string{
		"(A) 2026-01-02 Plan the launch +work id:1",
		"(B) Book the venue  p:1 id:2", // two spaces: htt rewrites only lines it changes
		"Call three venues p:2",
		"Write the announcement p:1 +work",
		"x 2026-01-03 2026-01-01 an old one, done in the file",
		"(C) unrelated due:friday",
		"orphan p:9",
	}
	writeLines(t, repo.ContextPath("work"), lines)
	before, _ := os.ReadFile(repo.ContextPath("work"))
	ctx, err := repo.Context("work")
	if err != nil {
		t.Fatal(err)
	}
	// Completing the last task: everything else stays as it was.
	if _, err := ctx.Complete(strconv.Itoa(len(lines)-1), domain.SubtasksAsk); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(repo.ContextPath("work"))
	if want := strings.TrimSuffix(string(before), "orphan p:9\n"); string(after) != want {
		t.Errorf("file:\n%s\nwant:\n%s", after, want)
	}
	done := readLines(t, repo.ContextPath(domain.DoneContextName))
	if len(done) != 1 || !strings.HasPrefix(done[0], "x "+time.Now().Format("2006-01-02")+" orphan p:9") {
		t.Errorf("done.txt: %q", done)
	}
}
