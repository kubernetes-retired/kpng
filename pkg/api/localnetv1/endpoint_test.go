package localnetv1

import (
	"fmt"
	"testing"
)

func TestInsertString(t *testing.T) {
	a := make([]string, 0)

	expect := func(exp string) {
		t.Helper()
		if s := fmt.Sprint(a); s != exp {
			t.Errorf("expected %s, got %s", exp, s)
		}
	}

	a = insertString(a, "b")
	expect("[b]")

	a = insertString(a, "d")
	expect("[b d]")

	a = insertString(a, "a")
	expect("[a b d]")

	a = insertString(a, "c")
	expect("[a b c d]")
}

func ExampleAddAddress() {
	e := &Endpoint{}

	e.AddAddress("1.1.1.2")
	e.AddAddress("1.1.1.4")
	e.AddAddress("1.1.1.3")
	e.AddAddress("1.1.1.1")
	e.AddAddress("1.1.1.2")

	e.AddAddress("::2")
	e.AddAddress("::4")
	e.AddAddress("::3")
	e.AddAddress("::1")
	e.AddAddress("::2")

	fmt.Println(e.IPsV4)
	fmt.Println(e.IPsV6)

	// Output:
	// [1.1.1.1 1.1.1.2 1.1.1.3 1.1.1.4]
	// [::1 ::2 ::3 ::4]
}
