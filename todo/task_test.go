package todo

import (
	"testing"
)

func stringsEqualOrFail(a string, b string) func(t *testing.T) {
	return func(t *testing.T) {
		if a != b {
			t.Errorf("['%s' != '%s']", a, b)
		}
	}

}

func TestTaskCreation(t *testing.T) {

	t.Run("Entry parsed for simple tasks",
		stringsEqualOrFail(NewTask("Hello World").Entry, "Hello World"))

	t.Run("A single tag is parsed",
		stringsEqualOrFail(NewTask("Hello World @foo").Tags["@"][0], "@foo"))

	t.Run("Annotations are absent from parsed entry",
		stringsEqualOrFail(NewTask("Hello tag:val World").Entry, "Hello World"))

	t.Run("Annotation values available",
		stringsEqualOrFail(NewTask("Hello tag:val World").Annotations["tag"], "val"))

	t.Run("Can change the priority",
		stringsEqualOrFail(NewTask("(B) hello world").SetPriority("A").ToString(), "(A) hello world"))

	t.Run("Can increase the priority",
		stringsEqualOrFail(NewTask("(B) hello world").IncreasePriority().ToString(), "(A) hello world"))

	t.Run("Can decrease the priority",
		stringsEqualOrFail(NewTask("(A) hello world").DecreasePriority().ToString(), "(B) hello world"))

	t.Run("Can't decrease the priority past minimum",
		stringsEqualOrFail(NewTask("(F) hello world").DecreasePriority().ToString(), "(F) hello world"))

	t.Run("Can't increase the priority past minimum",
		stringsEqualOrFail(NewTask("(A) hello world").IncreasePriority().ToString(), "(A) hello world"))

	t.Run("Can add an annotation",
		stringsEqualOrFail(NewTask("hello").Annotate("test", "123").ToString(), "hello test:123"))

	t.Run("Can remove an annotation",
		stringsEqualOrFail(NewTask("hello test:123").RemoveAnnotation("test").ToString(), "hello"))
}
