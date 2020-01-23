package main

import "fmt"

func ExampleIptRandom() {
	for _, v := range []struct{ idx, cnt int }{
		{0, 1},
		{0, 2}, {1, 2},
		{0, 3}, {1, 3}, {2, 3},
	} {
		fmt.Printf("%d,%d:%s\n", v.idx, v.cnt, iptRandom(v.idx, v.cnt))
	}

	// Output:
	// 0,1:
	// 0,2: -m statistic --mode random --probability 0.5000
	// 1,2:
	// 0,3: -m statistic --mode random --probability 0.3333
	// 1,3: -m statistic --mode random --probability 0.5000
	// 2,3:
}
