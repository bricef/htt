package todo

import (
	"fmt"
	"testing"
)

func TestStuff(t *testing.T) {
	examples := []string{
		"x (A) 2018-07-30 2018-07-31 Some todo item with +ProjectTag @atTag #hashtag and key:value pair",
		"(A) Thank Mom for the meatballs @phone",
		"(B) Schedule Goodwill pickup +GarageSale @phone",
		"Post signs around the neighborhood +GarageSale",
		"@GroceryStore Eskimo pies",
		"(A) Thank Mom for the meatballs @phone",
		"(B) Schedule Goodwill pickup +GarageSale @phone",
		"(B) Schedule Goodwill pickup +GarageSale @phone",
		"Post signs around the neighborhood +GarageSale",
		"Really gotta call Mom (A) @phone @someday",
		"(b) Get back to the boss",
		"(B)->Submit TPS report",
		"2011-03-02 Document +TodoTxt task format", // This is not a completion date because it's not complete
		"(A) 2011-03-02 Call Mom",
		"(A) Call Mom 2011-03-02",
		"(A) Call Mom +Family +PeaceLoveAndHappiness @iphone @phone",
		"Email SoAndSo at soandso@example.com",
		"Learn how to add 2+2",
		"x 2011-03-03 Call Mom", // this is a completion date
		"xylophone lesson",
		"X 2012-01-01 Make resolutions",
		"(A) x Find ticket prices",
		"x 2011-03-02 2011-03-01 Review Tim's pull request +TodoTxtTouch @github",
		"Some example with key:value and due:2010-01-02",
	}

	parser := NewTodoParser()

	for i, todo := range examples {
		fmt.Println()
		fmt.Printf("[%v]: %s\n", i, todo)
		node := parser.Parse(todo)
		parser.Prettyprint(node)
	}
}
