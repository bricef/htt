package utils

import (
	"bufio"
	"io"
	"log"
)

/*
 *	Logging helpers and prettifiers
 */
func DieOnError(message string, err error) {
	if err != nil {
		Fatal(message, err)
	}
}

func Info(args ...interface{}) {
	args = append([]interface{}{"ℹ️  "}, args...)
	log.Println(args...)
}

func Fatal(args ...interface{}) {
	args = append([]interface{}{"☠️  "}, args...)
	log.Fatal(args...)
}

func Failure(args ...interface{}) {
	args = append([]interface{}{"❌  "}, args...)
	log.Println(args...)
}

func Success(args ...interface{}) {
	args = append([]interface{}{"✅  "}, args...)
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
