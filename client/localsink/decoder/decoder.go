package decoder

import (
	"strings"

	"google.golang.org/protobuf/proto"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
)

type ServicesListener interface {
	// SetService is called when a service is added or updated
	SetService(service *localnetv1.Service)
	// DeleteService is called when a service is deleted
	DeleteService(namespace, name string)
}

type EndpointsListener interface {
	// SetEndpoint is called when an endpoint is added or updated
	SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint)
	// DeleteEndpoint is called when an endpoint is deleted
	DeleteEndpoint(namespace, serviceName, key string)
}

type Interface interface {
	// Sync signals an stream sync event
	Sync()

	// methods handling decoded values

	ServicesListener
	EndpointsListener

	// subset of localsink.Sink

	// Setup see localsink.Sink#Setup
	Setup()

	// WaitRequest see localsink.Sink#WaitRequest
	// XXX is ti really the place? specialized sinks mutating the node name are probably not the target here
	WaitRequest() (nodeName string, err error)

	// Reset see localsink.Sink#Reset
	Reset()
}

type Sink struct {
	Interface
}

var _ localsink.Sink = &Sink{}

func New(iface Interface) *Sink {
	return &Sink{iface}
}

func (s *Sink) Send(op *localnetv1.OpItem) (err error) {
	switch v := op.Op; v.(type) {
	case *localnetv1.OpItem_Set:
		set := op.GetSet()

		switch set.Ref.Set {
		case localnetv1.Set_ServicesSet:
			v := &localnetv1.Service{}

			err = proto.Unmarshal(set.Bytes, v)
			if err != nil {
				return
			}

			s.SetService(v)

		case localnetv1.Set_EndpointsSet:
			v := &localnetv1.Endpoint{}

			err = proto.Unmarshal(set.Bytes, v)
			if err != nil {
				return
			}

			parts := strings.Split(set.Ref.Path, "/")
			s.SetEndpoint(parts[0], parts[1], parts[2], v)

		default:
			return
		}

	case *localnetv1.OpItem_Delete:
		del := op.GetDelete()
		parts := strings.Split(del.Path, "/")

		switch del.Set {
		case localnetv1.Set_ServicesSet: // Service: namespace/name
			s.DeleteService(parts[0], parts[1])

		case localnetv1.Set_EndpointsSet: // Endpoint: namespace/name/key
			s.DeleteEndpoint(parts[0], parts[1], parts[2])

		default:
			// unknown set, ignore
		}

	case *localnetv1.OpItem_Sync:
		s.Sync()
	}

	return
}
