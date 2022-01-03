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

const TimestampLabel = "ts"

func CurrentLogFilePath() string {
	return LogFilePath(time.Now())
}

func LogFilePath(t time.Time) string {
	logFilePath := path.Join(vars.Get(vars.ConfigKeyDataDir), vars.DefaultTimelogDirName, t.Format("2006-01-02.log"))
	return logFilePath
}

func AddEntry(task *todo.Task) {
	now := time.Now().UTC()

	currentLog := CurrentLogFilePath()
	utils.EnsurePath(currentLog)

	f, err := os.OpenFile(currentLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	utils.DieOnError("Failed to open log file for writing: ", err)

	task.Annotate(TimestampLabel, now.Format(time.RFC3339))

	// start:
	// entryWithStart := fmt.Sprintf("start:%s %s \n", now.Format(time.RFC3339), strings.TrimSpace(entry))

	_, err = f.WriteString(fmt.Sprintf("%v\n", task.String()))
	fmt.Printf("Logging entry: %v\n", task.RemoveAnnotation(TimestampLabel).ColorString())
	utils.DieOnError("Failed to write entry to log", err)
}

func Show() {
	bytes, err := ioutil.ReadFile(CurrentLogFilePath())

	if err != nil {
		fmt.Fprintf(os.Stderr, "Today's timelog does not yet exist. Add an entry.")
	}

	print(string(bytes))
}

func ShowStatus() {
	currentTask := CurrentActive()
	if currentTask != nil {
		startedAt := currentTask.Annotations[TimestampLabel]
		startTime, err := time.Parse(time.RFC3339, startedAt)
		if err != nil {
			utils.Fatal("Failed to parse log entry.")
		}
		duration := utils.HumanizeDuration(time.Since(startTime))
		fmt.Printf("Currently working on: %v (%v) \n", currentTask.RemoveAnnotation(TimestampLabel).ColorString(), duration)
	} else {
		fmt.Printf("Not currently working on any task.\n")
	}
}

func CurrentActive() *todo.Task {
	lines := utils.ReadLines(CurrentLogFilePath())
	if len(lines) == 0 {
		return nil
	}
	t := todo.NewTask(lines[len(lines)-1])
	return t
}

func CurrentDuration() time.Duration {
	currentTask := CurrentActive()
	startedAt := currentTask.Annotations[TimestampLabel]
	startTime, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		utils.Fatal("Failed to parse log entry.")
	}
	return time.Since(startTime)

}
