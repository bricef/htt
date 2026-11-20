//go:build !windows

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestRepl_TheTUIInATerminal runs `htt repl` in a pseudo-terminal, as
// in a terminal window, and starts the TUI from it twice: /interactive,
// which moves to the next context and quits with q, then /i, quit with
// Ctrl-C. Each time the REPL is back at its prompt, in the context the
// TUI left, with its history, reading every key typed after.
func TestRepl_TheTUIInATerminal(t *testing.T) {
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	e.writeData("todo.txt", "groceries\n")
	e.writeData("work.txt", "standup\n")
	term := e.startInPty("repl")

	term.waitFor("htt(todo)> ")
	term.send("/interactive\r")
	term.waitFor(altScreenOn)
	term.waitFor("groceries") // the TUI is up, in todo
	term.send("l")            // the next context: work
	term.waitFor("standup")
	term.send("q")
	back := term.waitFor("htt(work)> ")
	// The TUI leaves its screen and mouse modes, and the REPL draws the
	// context the TUI moved to below the line typed, as after any
	// command.
	assertInOrder(t, "after q", back, altScreenOff, "(work): 1 tasks")
	for _, mode := range []string{"\x1b[?1002l", "\x1b[?1006l"} {
		assertContains(t, "after q", back, mode)
	}

	// Up brings back the line that started the TUI: the history is
	// intact. Down returns to the empty line.
	term.send("\x1b[A")
	term.waitFor("htt(work)> /interactive")
	term.send("\x1b[B")
	term.waitFor("htt(work)> ")

	term.send("/i\r")
	term.waitFor(altScreenOn)
	term.waitFor("standup")
	term.send("\x03") // Ctrl-C quits the TUI, as outside the REPL
	back = term.waitFor("htt(work)> ")
	assertInOrder(t, "after Ctrl-C", back, altScreenOff, "(work): 1 tasks")

	// Keys typed after the TUI reach the REPL, the first one included:
	// without its a, this line would be `dd from the repl`.
	term.send("add from the repl\r")
	term.waitFor("Added: " + today() + " from the repl")
	term.waitFor("htt(work)> ")
	term.send("q\r")
	term.exit()

	if strings.Contains(term.output(), "tui exited") {
		t.Errorf("the TUI ended with an error:\n%q", term.output())
	}
	assertEqual(t, "work.txt", e.readData("work.txt"), "standup\n"+today()+" from the repl\n")

	// Nothing left the terminal in raw mode: it echoes what's typed
	// again. Only on Linux, where the terminal outlives the session htt
	// led; BSD-derived systems, macOS among them, revoke it when htt
	// exits.
	if runtime.GOOS == "linux" {
		term.send("cooked\n")
		term.waitFor("cooked")
	}
}

const (
	altScreenOn  = "\x1b[?1049h"
	altScreenOff = "\x1b[?1049l"
)

// ptyTerm is htt running in a pseudo-terminal: what htt writes to the
// terminal collects in out, and send types keys.
type ptyTerm struct {
	t    *testing.T
	ptmx *os.File // the terminal's side: keys in, what htt writes out
	cmd  *exec.Cmd

	mu   sync.Mutex
	out  []byte
	seen int // how much of out waitFor has matched

	exited  chan struct{} // closed when htt has exited
	waitErr error         // how it exited
}

// startInPty starts htt with args in a 30x100 pseudo-terminal, its
// controlling terminal, as a shell starts it. The test keeps htt's side
// of the terminal open too, so the terminal outlives htt.
func (e *env) startInPty(args ...string) *ptyTerm {
	e.t.Helper()
	ptmx, tty, err := pty.Open()
	if err != nil {
		e.t.Fatalf("open a pseudo-terminal: %v", err)
	}
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 30, Cols: 100}); err != nil {
		e.t.Fatalf("size the pseudo-terminal: %v", err)
	}
	cmd := exec.Command(binaryPath, append([]string{"--no-color"}, args...)...)
	cmd.Env = append(append(e.homeEnv(), "PATH="+os.Getenv("PATH"), "TERM=xterm-256color"), e.extraEnv...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		e.t.Fatalf("start htt: %v", err)
	}
	p := &ptyTerm{t: e.t, ptmx: ptmx, cmd: cmd, exited: make(chan struct{})}
	go p.read()
	go func() {
		p.waitErr = cmd.Wait()
		close(p.exited)
	}()
	e.t.Cleanup(func() {
		_ = cmd.Process.Kill() // in case the test ended before htt did
		<-p.exited
		_ = ptmx.Close()
		_ = tty.Close()
	})
	return p
}

// read collects what htt writes, answering cursor position queries as
// a terminal does: htt's colour library asks at startup, and waits five
// seconds for an answer.
func (p *ptyTerm) read() {
	buf := make([]byte, 4096)
	answered := 0
	for {
		n, err := p.ptmx.Read(buf)
		p.mu.Lock()
		p.out = append(p.out, buf[:n]...)
		asked := bytes.Count(p.out, []byte("\x1b[6n"))
		p.mu.Unlock()
		for ; answered < asked; answered++ {
			_, _ = p.ptmx.Write([]byte("\x1b[1;1R"))
		}
		if err != nil {
			return
		}
	}
}

// waitFor waits for want in what htt writes after the last thing waited
// for, and returns what htt wrote up to and including it.
func (p *ptyTerm) waitFor(want string) string {
	p.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		p.mu.Lock()
		rest := string(p.out[p.seen:])
		if i := strings.Index(rest, want); i >= 0 {
			p.seen += i + len(want)
			p.mu.Unlock()
			return rest[:i+len(want)]
		}
		p.mu.Unlock()
		if time.Now().After(deadline) {
			p.t.Fatalf("timed out waiting for %q; after the last thing waited for, htt wrote:\n%q", want, rest)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// send types keys.
func (p *ptyTerm) send(keys string) {
	p.t.Helper()
	if _, err := p.ptmx.Write([]byte(keys)); err != nil {
		p.t.Fatalf("type %q: %v", keys, err)
	}
}

// exit waits for htt to exit, and fails the test unless it exits 0.
func (p *ptyTerm) exit() {
	p.t.Helper()
	select {
	case <-p.exited:
		if p.waitErr != nil {
			p.t.Fatalf("htt: %v; it wrote:\n%q", p.waitErr, p.output())
		}
	case <-time.After(10 * time.Second):
		p.t.Fatalf("htt didn't exit; it wrote:\n%q", p.output())
	}
}

// output is everything htt has written.
func (p *ptyTerm) output() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return string(p.out)
}
