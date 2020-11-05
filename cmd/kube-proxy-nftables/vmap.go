package main

import "fmt"

func vmapAdd(chain *chainBuffer, match string, kv string) {
	if chain.Len() == 0 {
		fmt.Fprintf(chain, "  %s vmap { ", match)
		chain.Defer(func(c *chainBuffer) {
			fmt.Fprintln(c, "}")
		})
	} else {
		chain.Write([]byte(", "))
	}
	fmt.Fprint(chain, kv)
}
