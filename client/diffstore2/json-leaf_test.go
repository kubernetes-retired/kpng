package diffstore2

func ExampleJSONLeafStore() {
    type myT struct {
        V string
    }

	store := NewJSONStore[string, myT]()

	store.Get("a").Set(myT{"a1"})
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set(myT{"a1"})
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set(myT{"a2"})
	store.Done()
	store.printDiff()

	store.Reset()
	store.Done()
	store.printDiff()

	// Output:
	// -----
	// C a => "{\"V\":\"a1\"}"
	// -----
	// <same>
	// -----
	// U a => "{\"V\":\"a2\"}"
	// -----
	// D a
}
