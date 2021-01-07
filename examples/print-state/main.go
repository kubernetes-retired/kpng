package main

import (
	"fmt"
	"os"
	"time"

	"m.cluseau.fr/kube-proxy2/pkg/client"
)

func main() {
	client.Run(nil, printState)
}

func printState(items []*client.ServiceEndpoints) {
	fmt.Print("\033[H\033[2J\033[3J")
	fmt.Println("#", time.Now())
	for _, item := range items {
		fmt.Fprintln(os.Stdout, item)
	}
}
