package pipe

import (
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/localsink"
)

type Sink struct {
	targetSinks []localsink.Sink
	buffer      []*client.ServiceEndpoints
	localsink.Config
}

func New(targetSinks ...localsink.Sink) *Sink {
	return &Sink{
		targetSinks: targetSinks,
	}
}

func (ps *Sink) Reset() {
	for _, sink := range ps.targetSinks {
		sink.Reset()
	}
}

func (ps *Sink) Setup() {
	for _, sink := range ps.targetSinks {
		sink.Setup()
	}
}

func (ps *Sink) Send(op *localnetv1.OpItem) error {
	for _, sink := range ps.targetSinks {
		if err := sink.Send(op); err != nil {
			return err
		}
	}
	return nil
}
