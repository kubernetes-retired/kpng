package diffstore2

import (
    "fmt"
)

func (store *Store[K,V]) printDiff() {
	fmt.Println("-----")
	hasChanges := false

	for _, i := range store.Changed() {
		hasChanges = true

		s := "U"
		if i.Created() {
			s = "C"
		}
		fmt.Printf("%s %v => %q\n", s, i.Key(), fmt.Sprint(i.Value()))
	}

	for _, i := range store.Deleted() {
		hasChanges = true

		fmt.Printf("D %v\n", i.Key())
	}

	if !hasChanges {
		fmt.Println("<same>")
	}
}
