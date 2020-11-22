package diffstore

import (
	"fmt"

	"github.com/cespare/xxhash"
)

func ExampleDiffStore() {
	s := New()

	set := func(k, v string) {
		fmt.Printf("set %v to %s\n", k, v)
		s.Set([]byte(k), xxhash.Sum64([]byte(v)), v)
	}

	set("a", "alice")
	set("b", "bob")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// --------------------------------------------------------------------------
	fmt.Println()
	s.Reset(ItemUnchanged)

	set("a", "alice2")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// --------------------------------------------------------------------------
	fmt.Println()
	s.Reset(ItemDeleted)

	set("a", "alice3")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// --------------------------------------------------------------------------
	fmt.Println()
	s.Reset(ItemDeleted)

	set("a", "alice3")
	set("b", "bob")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// double reset will remove all entries (and should not crash)
	s.Reset(ItemDeleted)
	s.Reset(ItemDeleted)

	fmt.Println("tree size after double reset:", s.tree.Len())

	// Output:
	// set a to alice
	// set b to bob
	// -> updated: [{[97] alice} {[98] bob}] deleted: []
	//
	// set a to alice2
	// -> updated: [{[97] alice2}] deleted: []
	//
	// set a to alice3
	// -> updated: [{[97] alice3}] deleted: [{[98] <nil>}]
	//
	// set a to alice3
	// set b to bob
	// -> updated: [{[98] bob}] deleted: []
	// tree size after double reset: 0
}
