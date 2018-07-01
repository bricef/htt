package timelogs

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/hypotheticalco/tracker-client/vars"

	"github.com/hypotheticalco/tracker-client/utils"
)

func currentLogFilePath(now time.Time) string {
	logFilePath := path.Join(vars.Get(vars.ConfigKeyDataDir), vars.DefaultTimelogDirName, now.Format("2006-01-02.log"))
	return logFilePath
}

func AddEntry(entry string) {
	now := time.Now()

	currentLog := currentLogFilePath(now)
	utils.EnsurePath(currentLog)

	f, err := os.OpenFile(currentLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	utils.DieOnError("Failed to open log file for writing: ", err)

	// start:
	entryWithStart := fmt.Sprintf("start:%s %s \n", now.Format("2006-01-02T15:04:05"), strings.TrimSpace(entry))

	_, err = f.WriteString(entryWithStart)
	utils.DieOnError("Failed to write entry to log", err)
}
