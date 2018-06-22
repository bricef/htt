package todo

import (
	"log"
	"os"
	"path"
	"strings"

	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/hypotheticalco/tracker-client/vars"
)

/*
 * Util functions
 */

func ensurePath(filename string) {
	err := os.MkdirAll(path.Dir(filename), 0700)
	utils.DieOnError("Could not ensure path "+path.Dir(filename)+": ", err)
}

func todoFilePath() string {
	path := path.Join(vars.Get(vars.ConfigKeyDataDir), vars.Get(vars.ConfigKeyTodoFileName))
	return path
}

func todoFile() *os.File {
	todoFile := todoFilePath()
	ensurePath(todoFile)
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
	UID   string
	Entry string
	Tags  []string
}

// AddTodo will add an item to the default todo list
func AddTodo(todo string) {
	f := todoFile()

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
	return tasks
}
