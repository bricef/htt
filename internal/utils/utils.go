package utils

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"regexp/syntax"
	"strings"
	"time"

	"github.com/buger/goterm"
)

// StringToFilename will convert an arbitrary string into a valid filename
func StringToFilename(raw string) string {
	return strings.Map(func(c rune) rune {
		if syntax.IsWordChar(c) {
			return c
		} else {
			return '_'
		}
	}, raw)
}

/*
 *	Logging helpers and prettifiers
 */

func DieOnError(message string, err error) {
	if err != nil {
		Fatal(message, err)
	}
}

func Info(args ...interface{}) {
	args = append([]interface{}{"ℹ️ "}, args...)
	log.Println(args...)
}

func Fatal(args ...interface{}) {
	args = append([]interface{}{"❌ "}, args...)
	log.Fatal(args...)
}

func Failure(args ...interface{}) {
	args = append([]interface{}{"❌ "}, args...)
	log.Println(args...)
}

func Success(args ...interface{}) {
	args = append([]interface{}{"✅ "}, args...)
	log.Println(args...)
}

func Warning(args ...interface{}) {
	args = append([]interface{}{"⚠️ "}, args...)
	log.Println(args...)
}

/*
 * 	Line scanner that keeps track of which line
 */
type LineScanner struct {
	*bufio.Scanner
	Line int
}

func NewLineScanner(reader io.Reader) *LineScanner {
	return &LineScanner{bufio.NewScanner(reader), 0}
}

func (l *LineScanner) Scan() bool {
	ok := l.Scanner.Scan()
	if ok {
		l.Line++
	}
	return ok
}

func ReadLines(filepath string) []string {
	// short circuit on file missing
	_, err := os.Stat(filepath)
	if err != nil {
		return []string{}
	}

	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	DieOnError("Could not open file while reading lines: ", err)

	var lines []string

	scanner := NewLineScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(string(scanner.Text()))
		lines = append(lines, line)
	}
	return lines

}

func EnsurePath(filename string) {
	err := os.MkdirAll(path.Dir(filename), 0700)
	DieOnError("Could not ensure path "+path.Dir(filename)+": ", err)
}

func StringSliceIndex(slice []string, item string) (int, error) {
	for i, s := range slice {
		if s == item {
			return i, nil
		}
	}
	return 0, errors.New("Could not find item in slice")
}

func EditFilePath(filepath string) {
	editor, ok := os.LookupEnv("EDITOR")
	if !ok || editor == "" {
		Fatal("$EDITOR variable is empty or not set. Could not edit task.")
	}

	proc := exec.Command(editor, filepath)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	err := proc.Start()
	DieOnError("Failed to start the editor: ", err)

	err = proc.Wait()
	DieOnError("Error running editor: ", err)
}

func ClearScreen() {
	goterm.Clear()
	goterm.MoveCursor(1, 1)
	goterm.Flush()
}

func HumanizeDuration(d time.Duration) string {
	if d.Minutes() < 1 {
		return d.Round(time.Second).String()
	} else if d.Hours() < 1 {
		return fmt.Sprintf("%vm", d.Round(time.Minute).Minutes())
	} else if d.Hours() < 24 {
		return fmt.Sprintf("%vh%vm", d.Truncate(time.Hour).Hours(), math.Mod(d.Truncate(time.Minute).Minutes(), 60.0))
	}
	return d.String()
}
