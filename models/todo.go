package models

import (
	"bufio"
	"log"
	"os"
	"path"

	"github.com/hypotheticalco/tracker-client/vars"
)

/*
 * Util functions
 */

func ensurePath(filename string) {
	err := os.MkdirAll(path.Dir(filename), 0700)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func todoFilePath() string {
	return path.Join(vars.Get(vars.ConfigKeyDataDir), vars.Get(vars.DefaultTodoFileName))
}

func todoFile() (*os.File, error) {
	// Get todofile and make sure the path exists
	todoFile := todoFilePath()
	ensurePath(todoFile)
	f, err := os.OpenFile(todoFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	return f, err
}

func getTags(entry string) []string {
	return []string{}
}

func getPrefixedWords(prefix string, entry string) []string {
	return []string{}
}

func taskListFromFile(filename string) []Task {
	f, err := todoFile()
	if err != nil {
		log.Fatal(err)
	}
	var tasks []Task

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry := string(scanner.Text())
		tasks = append(tasks, Task{
			ID:    1,
			Entry: entry,
			Tags:  getTags(entry),
		})
	}
	return []Task{}
}

// Task represents a task in a todo list.
// It's needed because we need to have an ID
// for tasks that is stable even though they might be filtered.
type Task struct {
	ID    int
	Entry string
	Tags  []string
}

// AddTodo will add an item to the default todo list
func AddTodo(todo string) {
	f, err := todoFile()
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	_, err = f.WriteString(todo + "\n")
	if err != nil {
		log.Fatal(err)
	}
}

// GetTodos will add the todos according to the serach terms
func GetTodos(searchTerms []string) []Task {
	tasks := taskListFromFile(todoFilePath())

	return tasks
}
