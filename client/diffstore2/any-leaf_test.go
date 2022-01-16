package diffstore2

func ExampleAnyLeafStore() {
	store := NewAnyStore[string](func(a, b string) bool { return a == b })

	store.Get("a").Set("a1")
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set("a1")
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set("a2")
	store.Done()
	store.printDiff()

	store.Reset()
	store.Done()
	store.printDiff()

	// Output:
	// -----
	// C a => "{a1}"
	// -----
	// <same>
	// -----
	// U a => "{a2}"
	// -----
	// D a
}
