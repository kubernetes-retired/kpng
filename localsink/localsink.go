package localsink

import "sigs.k8s.io/kpng/pkg/server/watchstate"

type Sink interface {
	// WaitRequest waits for the next diff request, returning the requested node name. If an error is returned, it will cancel the job.
	WaitRequest() (nodeName string, err error)

	watchstate.OpSink
}
