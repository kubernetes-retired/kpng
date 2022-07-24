package diffstore

import (
	"fmt"
	"math"
)

func ExampleBufferLeaf() {
	bl := NewBufferLeaf()

	fmt.Fprintf(bl, "hello %d %.2f", 1, math.Pi)

	fmt.Println("[", bl, "]")

	// Output:
	// [ hello 1 3.14 ]
}
