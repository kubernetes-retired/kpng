package serviceevents

import (
	"fmt"

	"sigs.k8s.io/kpng/api/localnetv1"
)

type wrapperBackend struct{}

func (_ wrapperBackend) Sync()                                  { fmt.Println("backend Sync") }
func (_ wrapperBackend) Setup()                                 { fmt.Println("backend Setup") }
func (_ wrapperBackend) Reset()                                 { fmt.Println("backend Reset") }
func (_ wrapperBackend) SetService(service *localnetv1.Service) { fmt.Println("backend SetService") }
func (_ wrapperBackend) DeleteService(namespace, name string)   { fmt.Println("backend DeleteService") }
func (_ wrapperBackend) WaitRequest() (nodeName string, err error) {
	fmt.Println("backend WaitRequest")
	return "localhost", nil
}
func (_ wrapperBackend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	fmt.Println("backend SetEndpoint")
}
func (_ wrapperBackend) DeleteEndpoint(namespace, serviceName, key string) {
	fmt.Println("backend DeleteEndpoint")
}

func ExampleWrap() {
	w := Wrap(wrapperBackend{})

	w.Setup()
	w.Reset()
	w.Sync()

	// Output:
	// backend Setup
	// backend Reset
	// backend Sync
}
