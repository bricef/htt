package todo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bricef/htt/internal/utils"
	"github.com/bricef/htt/internal/vars"
)

/*
 * Util functions
 */

func SetCurrentContext(raw string) {
	context := utils.StringToFilename(raw)
	if context == "" {
		utils.Fatal("Can't use an empty context.")
	}
	contextStoreFilePath := path.Join(vars.Get(vars.ConfigKeyTrackerDir), vars.DefaultContextFileName)

	err := ioutil.WriteFile(contextStoreFilePath, []byte(context), 0666)
	utils.DieOnError("Failed to open context persistance file: ", err)
}

func GetCurrentContext() *Context {
	contextStoreFilePath := path.Join(vars.Get(vars.ConfigKeyTrackerDir), vars.DefaultContextFileName)
	context := vars.DefaultContext
	if _, err := os.Stat(contextStoreFilePath); err == nil {
		content, err := ioutil.ReadFile(contextStoreFilePath)
		utils.DieOnError("Failed to open context persistance file: ", err)
		context = strings.TrimSpace(string(content))
	}
	return NewContext(context).Read()
}

func taskListFromFile(filename string) []*Task {
	utils.EnsurePath(filename)
	lines := utils.ReadLines(filename)
	var tasks []*Task

	for i, line := range lines {
		if line != "" {
			t := NewTask(line)
			t.Line = i
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// TODO: Implement
func terms2predicate(terms []string) func(*Task) bool {
	utils.Warning("Parsing search terms is not yet supported.")
	return func(t *Task) bool { return true }
}

func ShowStatus() {
	current := GetCurrentContext()
	contexts := GetContexts()

	fmt.Printf("Available Contexts: ")
	for _, c := range contexts {
		fmt.Printf("%s ", c.ConsoleString())
	}
	fmt.Println()

	fmt.Printf("Current Context: %s\n", current.ConsoleString())

	current.Show()
}

func GetContexts() []*Context {
	files, err := os.ReadDir(vars.Get(vars.ConfigKeyDataDir))
	utils.DieOnError("Failed to list contexts", err)

	contexts := []*Context{}
	for _, file := range files {
		filename := file.Name()
		if !file.IsDir() && strings.HasSuffix(filename, vars.DefaultFileExtension) && filename != "done.txt" {
			context := strings.TrimSuffix(filename, vars.DefaultFileExtension)
			contexts = append(contexts, NewContext(context))
		}
	}
	return contexts
}

func Move(t *Task, old *Context, new *Context) error {
	err := old.Remove(t)
	if err != nil {
		return err
	}
	new.Add(t)
	return nil
}

func CompleteTask(strid string) (*Task, error) {
	current := GetCurrentContext()
	done := NewContext("done")
	t, err := current.GetTaskByStrId(strid)
	if err != nil {
		return nil, err
	}

	t.Do(current, time.Now())
	err = Move(t, current, done)
	if err != nil {
		return nil, err
	}

	err = current.Sync()
	if err != nil {
		return nil, err
	}

	err = done.Sync()
	if err != nil {
		return nil, err
	}

	return t, nil
}
