package fullstate

import (
	"github.com/google/btree"
	"google.golang.org/protobuf/proto"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
)

type ServiceEndpoints struct {
	Service   *localnetv1.Service
	Endpoints []*localnetv1.Endpoint
}

type Callback func(item <-chan *ServiceEndpoints)

// EndpointsClient is a simple client to kube-proxy's Endpoints API.
type Sink struct {
	Config   *localsink.Config
	Callback Callback

	data *btree.BTree
}

func New(config *localsink.Config) *Sink {
	return &Sink{
		Config: config,
		data:   btree.New(2),
	}
}

var _ localsink.Sink = &Sink{}

// ArrayCallback wraps a array callback
func ArrayCallback(callback func([]*ServiceEndpoints)) Callback {
	items := make([]*ServiceEndpoints, 0)

	return func(ch <-chan *ServiceEndpoints) {
		items = items[:0]

		for seps := range ch {
			items = append(items, seps)
		}

		callback(items)

		return
	}
}

func (s *Sink) Setup() { /* noop */ }

func (s *Sink) WaitRequest() (nodeName string, err error) {
	return s.Config.NodeName, nil
}

func (s *Sink) Reset() {
	s.data.Clear(false)
}

func (s *Sink) Send(op *localnetv1.OpItem) (err error) {
	switch v := op.Op; v.(type) {
	case *localnetv1.OpItem_Set:
		set := op.GetSet()

		var v proto.Message
		switch set.Ref.Set {
		case localnetv1.Set_ServicesSet:
			v = &localnetv1.Service{}
		case localnetv1.Set_EndpointsSet:
			v = &localnetv1.Endpoint{}

		default:
			return
		}

		err = proto.Unmarshal(set.Bytes, v)
		if err != nil {
			return
		}

		s.data.ReplaceOrInsert(kv{set.Ref.Path, v})

	case *localnetv1.OpItem_Delete:
		s.data.Delete(kv{Path: op.GetDelete().Path})

	case *localnetv1.OpItem_Sync:
		results := make(chan *ServiceEndpoints, 1)

		go func() {
			defer close(results)

			var seps *ServiceEndpoints

			s.data.Ascend(func(i btree.Item) bool {
				switch v := i.(kv).Value.(type) {
				case *localnetv1.Service:
					if seps != nil {
						results <- seps
					}

					seps = &ServiceEndpoints{Service: v}
				case *localnetv1.Endpoint:
					seps.Endpoints = append(seps.Endpoints, v)
				}

				return true
			})

			if seps != nil {
				results <- seps
			}
		}()

		s.Callback(results)
	}

	return
}
