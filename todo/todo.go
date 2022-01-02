package todo

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bricef/htt/utils"
	"github.com/bricef/htt/vars"
)

/*
 * Util functions
 */

func SetContext(raw string) {
	context := utils.StringToFilename(raw)
	if context == "" {
		utils.Fatal("Can't use an empty context.")
	}
	contextStoreFilePath := path.Join(vars.Get(vars.ConfigKeyDataDir), vars.DefaultContextFileName)

	err := ioutil.WriteFile(contextStoreFilePath, []byte(context), 0666)
	utils.DieOnError("Failed to open context persistance file: ", err)
}

func ContextToFilePath(context string) string {
	return path.Join(vars.Get(vars.ConfigKeyDataDir), context+vars.DefaultFileExtension)
}

func GetCurrentContext() string {
	contextStoreFilePath := path.Join(vars.Get(vars.ConfigKeyDataDir), vars.DefaultContextFileName)
	context := vars.DefaultContext

	if _, err := os.Stat(contextStoreFilePath); err == nil {
		content, err := ioutil.ReadFile(contextStoreFilePath)
		utils.DieOnError("Failed to open context persistance file: ", err)
		context = strings.TrimSpace(string(content))
	}

	return context
}

func todoFilePath() string {
	path := path.Join(vars.Get(vars.ConfigKeyDataDir), GetCurrentContext()+vars.DefaultFileExtension)
	return path
}

func todoFile() *os.File {
	todoFile := todoFilePath()
	utils.EnsurePath(todoFile)
	f, err := os.OpenFile(todoFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	utils.DieOnError("Could not open file: "+todoFile+": ", err)

	return f
}
func contextFile(context string) *os.File {
	todoFile := ContextToFilePath(context)
	utils.EnsurePath(todoFile)
	f, err := os.OpenFile(todoFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	utils.DieOnError("Could not open file: "+todoFile+": ", err)

	return f
}

func getTags(entry string) []string {
	return getPrefixedWords("#", entry)
}

// TODO: Implement
func getPrefixedWords(prefix string, entry string) []string {
	return []string{}
}

func taskListFromFile(filename string) []*Task {
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

// AddTodo will add an item to the default todo list
func AddTodo(todo string) {
	AddToContext(GetCurrentContext(), todo)
}

func AddToContext(context string, todo string) {
	f := contextFile(context)

	defer f.Close()

	_, err := f.WriteString(todo + "\n")
	if err != nil {
		log.Fatal(err)
	}
}

// GetTodos will add the todos according to the serach terms
func GetTodos() []*Task {
	return GetTodosForContext(GetCurrentContext())
}

func GetTodosForContext(context string) []*Task {
	return taskListFromFile(ContextToFilePath(context))
}

// Filter will filter out any tasks that do not match the serach terms
// More than one search term can be given
// TODO: Implement
func FilterTasks(tasks []*Task, predicate func(*Task) bool) []*Task {
	ts := []*Task{}
	for _, t := range tasks {
		if predicate(t) {
			ts = append(ts, t)
		}
	}
	return ts
}

// GetTodoID will get a given todo by ID (ID is an idex from 1)
func GetTodoID(index int) *Task {
	ts := GetTodos()
	if index > len(ts) || index < 0 {
		utils.Fatal("Item selected is outside of range")
	}
	return ts[index]
}

func setTodos(tasks []*Task) {
	originalPath := todoFilePath()
	backupPath := originalPath + ".bak"
	err := os.Rename(originalPath, backupPath)
	utils.DieOnError("Could not create a backup file.", err)

	f := todoFile()
	defer f.Close()
	for _, task := range tasks {
		_, err := f.WriteString(task.ToString() + "\n")
		utils.DieOnError("Failed to write todo to file", err)
	}
}

// Delete will remove the given task from the task list
func Delete(task *Task) {
	todos := GetTodos()
	newTodos := append(todos[:task.Line], todos[task.Line+1:]...)
	setTodos(newTodos)
}

//TODO: Implement
func terms2predicate(terms []string) func(*Task) bool {
	utils.Warning("Parsing search terms is not yet supported.")
	return func(t *Task) bool { return true }
}

// Show will print out the tasks given
func Show(context string, terms []string) {
	ts := GetTodosForContext(context)
	tasks := []*Task{}

	if terms != nil && len(terms) > 0 {
		tasks = FilterTasks(ts, terms2predicate(terms))
	} else {
		tasks = ts
	}

	fmt.Println("")
	for _, todo := range tasks {
		if vars.GetBool(vars.ConfigKeyDisableColor) {
			fmt.Printf("%3d %s\n", todo.Line, todo.ToString())
		} else {
			fmt.Printf("%3d %s\n", todo.Line, todo.ColorString())
		}
	}
	fmt.Printf("\n--- (%s): %d of %d tasks shown ---\n", context, len(tasks), len(ts))
}

func Replace(id int, t *Task) {
	todos := GetTodos()
	todos[id] = t
	setTodos(todos)
}

func GetContexts() []string {
	fileinfos, err := ioutil.ReadDir(vars.Get(vars.ConfigKeyDataDir))
	utils.DieOnError("Failed to list contexts", err)

	contexts := []string{}
	for _, info := range fileinfos {
		filename := info.Name()
		if strings.HasSuffix(filename, vars.DefaultFileExtension) && filename != "done.txt" {
			context := strings.TrimSuffix(filename, vars.DefaultFileExtension)
			contexts = append(contexts, context)
		}
	}
	return contexts
}

func DoneFilePath() string {
	return path.Join(vars.Get(vars.ConfigKeyDataDir), vars.DefaultDoneFileName+vars.DefaultFileExtension)
}

func appendDone(t *Task) {
	doneFilePath := DoneFilePath()

	f, err := os.OpenFile(doneFilePath, os.O_APPEND|os.O_WRONLY, 0600)
	utils.DieOnError("Failed to open done file for writing: ", err)

	_, err = f.WriteString(strings.TrimSpace(t.ToString()) + "\n")
	utils.DieOnError("Failed to archive todo", err)
}

func CompleteTask(id int) {
	t := GetTodoID(id)
	doneEntry := t.Do(GetCurrentContext(), time.Now())
	fmt.Printf("Completed: %s\n", t.ToString())
	appendDone(doneEntry) // append before delete so we don't loose data.
	Delete(t)
}
