// Package server contains every kpng's function related to serving its API.
//
// The general approach is that you want to provide a [proxystore.Store] and make something of it.
// This store contains what we call the global model, representing the cluster-wide information
// needed by kpng to provide what we call the local model, representing the node-level information.
//
// There are 3 ways to provide a [proxystore.Store], all using jobs:
//   - [jobs/kube2store]: from a k8s' apiserver;
//   - [jobs/api2store]: from another kpng instance serving the global API;
//   - [jobs/file2store]: from a serialized global model.
//
// From a [proxystore.Store], you can:
//   - serve it over the kpng's API ([jobs/store2api]);
//   - serialize the global model to a file ([jobs/store2file]);
//   - stream the changes to the global model ([jobs/store2globaldiff]);
//   - stream the changes to the local model ([jobs/store2localdiff]).
//
// The main goal of kpng is to provide a local model. Currently, the local model is provided as
// a stream of changes. This stream is sent over the [localsink.Sink] interface.
//
// There are 2 ways to get the local model's stream:
//   - externaly from an kpng API ([jobs/api2local])
//   - internaly from a store ([jobs/store2localdiff]).
//
// [proxystore.Store]: https://pkg.go.dev/sigs.k8s.io/kpng/server/proxystore#Store
// [jobs/kube2store]: https://pkg.go.dev/sigs.k8s.io/kpng/server/jobs/kube2store
// [jobs/api2store]: https://pkg.go.dev/sigs.k8s.io/kpng/server/jobs/api2store
// [jobs/file2store]: https://pkg.go.dev/sigs.k8s.io/kpng/server/jobs/file2store
// [jobs/store2api]: https://pkg.go.dev/sigs.k8s.io/kpng/server/jobs/store2api
// [jobs/store2file]: https://pkg.go.dev/sigs.k8s.io/kpng/server/jobs/store2file
// [jobs/store2globaldiff]: https://pkg.go.dev/sigs.k8s.io/kpng/server/jobs/store2globaldiff
// [jobs/store2localdiff]: https://pkg.go.dev/sigs.k8s.io/kpng/server/jobs/store2localdiff
// [localsink.Sink]: https://pkg.go.dev/sigs.k8s.io/kpng/client/localsink#Sink
package server
