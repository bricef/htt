package utils

import (
	"bufio"
	"errors"
	"io"
	"log"
	"os"
	"path"
	"regexp/syntax"
	"strings"
)

// StringTofilename will convert an arbitrary string into a valid filename
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
	args = append([]interface{}{"☠️ "}, args...)
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
