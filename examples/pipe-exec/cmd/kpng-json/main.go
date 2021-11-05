package main

import (
	"encoding/json"
	"os"

	"sigs.k8s.io/kpng/client"
)

func main() {
	client.Run(jsonPrint)
}

func jsonPrint(items []*client.ServiceEndpoints) {
	enc := json.NewEncoder(os.Stdout)
	for _, item := range items {
		_ = enc.Encode(item)
	}
}
