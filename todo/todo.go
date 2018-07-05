package todo

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/hypotheticalco/tracker-client/vars"
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

func taskListFromFile(filename string) []Task {
	f := todoFile()

	var tasks []Task
	scanner := utils.NewLineScanner(f)
	for scanner.Scan() {
		entry := strings.TrimSpace(string(scanner.Text()))
		if entry != "" {
			tasks = append(tasks, Task{
				Line:  scanner.Line,
				Entry: entry,
				Tags:  getTags(entry),
			})
		}
	}
	return tasks
}

// Task represents a task in a todo list.
// for tasks that is stable even though they might be filtered.
type Task struct {
	Line  int
	Entry string
	Tags  []string
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
func GetTodos() []Task {
	return taskListFromFile(todoFilePath())
}

// Filter will filter out any tasks that do not match the serach terms
// More than one search term can be given
// TODO: Implement
func Filter(tasks []Task, terms []string) []Task {
	utils.Failure("`todo.Filter` is not Implemented. Returning unfiltered list.")
	return tasks
}

// GetTodoID will get a given todo by ID (ID is an idex from 1)
func GetTodoID(index int) Task {
	ts := GetTodos()
	if index > len(ts) {
		utils.Fatal("Item selected is outside of range")
	}
	return ts[index-1]
}

func setTodos(tasks []Task) {
	originalPath := todoFilePath()
	backupPath := originalPath + ".bak"
	err := os.Rename(originalPath, backupPath)
	utils.DieOnError("Could not create a backup file.", err)

	f := todoFile()
	defer f.Close()
	for _, task := range tasks {
		_, err := f.WriteString(task.Entry + "\n")
		utils.DieOnError("Failed to write todo to file", err)
	}
}

// Delete will remove the given task from the task list
func Delete(task Task) {
	todos := GetTodos()
	newTodos := append(todos[:task.Line-1], todos[task.Line:]...)
	setTodos(newTodos)
}

// Show will print out the tasks given
func Show(tasks []Task) {
	ts := GetTodos()
	for _, todo := range tasks {
		fmt.Printf("%3d %s\n", todo.Line, todo.Entry)
	}
	fmt.Printf("--- (%s): %d of %d tasks shown ---\n", GetCurrentContext(), len(tasks), len(ts))
}

func Replace(id int, entry string) {
	todos := GetTodos()
	todos[id-1].Entry = entry
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

func appendDone(entry string) {
	doneFilePath := path.Join(vars.Get(vars.ConfigKeyDataDir), vars.DefaultDoneFileName+vars.DefaultFileExtension)

	f, err := os.OpenFile(doneFilePath, os.O_APPEND|os.O_WRONLY, 0600)
	utils.DieOnError("Failed to open done file for writing: ", err)

	_, err = f.WriteString(strings.TrimSpace(entry) + "\n")
	utils.DieOnError("Failed to archive todo", err)
}

func CompleteTask(id int) {
	t := GetTodoID(id)
	context := GetCurrentContext()
	doneEntry := fmt.Sprintf("x %s %s context:%s", time.Now().Format("2006-01-02"), t.Entry, context)
	fmt.Printf("Completed: %s\n", t.Entry)
	appendDone(doneEntry) // append before delete so we don't loose data.
	Delete(t)
}
