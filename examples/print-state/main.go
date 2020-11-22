package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mcluseau/kube-proxy2/pkg/client"
)

func main() {
	client.Run(nil, printState)
}

func printState(items []*client.ServiceEndpoints) {
	fmt.Fprintln(os.Stdout, "#", time.Now())
	for _, item := range items {
		fmt.Fprintln(os.Stdout, item)
	}
}
