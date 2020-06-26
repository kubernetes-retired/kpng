package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/client"
)

func main() {
	client.Run(printState)
}

func printState(items []*localnetv1.ServiceEndpoints) {
	fmt.Fprintln(os.Stdout, "#", time.Now())
	for _, item := range items {
		fmt.Fprintln(os.Stdout, item)
	}
}
