package timelogs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/bricef/htt/todo"

	"github.com/bricef/htt/vars"

	"github.com/bricef/htt/utils"
)

func CurrentLogFilePath() string {
	return LogFilePath(time.Now())
}

func LogFilePath(t time.Time) string {
	logFilePath := path.Join(vars.Get(vars.ConfigKeyDataDir), vars.DefaultTimelogDirName, t.Format("2006-01-02.log"))
	return logFilePath
}

func AddEntry(entry string) {
	now := time.Now().UTC()

	currentLog := CurrentLogFilePath()
	utils.EnsurePath(currentLog)

	f, err := os.OpenFile(currentLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	utils.DieOnError("Failed to open log file for writing: ", err)

	task := todo.NewTask(entry)
	task.Annotate("start", now.Format(time.RFC3339))

	// start:
	// entryWithStart := fmt.Sprintf("start:%s %s \n", now.Format(time.RFC3339), strings.TrimSpace(entry))

	_, err = f.WriteString(fmt.Sprintf("%v\n", task.ToString()))
	utils.DieOnError("Failed to write entry to log", err)
}

func Show() {
	bytes, err := ioutil.ReadFile(CurrentLogFilePath())
	utils.DieOnError("Failed to read today's log file. Could be you haven't created an entry yet. ", err)

	print(string(bytes))
}

func CurrentActive() *todo.Task {
	lines := utils.ReadLines(CurrentLogFilePath())
	if len(lines) == 0 {
		return nil
	}
	t := todo.NewTask(lines[len(lines)-1])
	return t
}
