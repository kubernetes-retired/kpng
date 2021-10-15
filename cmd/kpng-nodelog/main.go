/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/localsink"
)

var (
	cfg = &localsink.Config{}
)

func main() {
	r := &client.Runner{}

	cmd := &cobra.Command{
		Run: func(_ *cobra.Command, _ []string) {
			r.RunSink(&sink{})
		},
	}

	flags := cmd.Flags()
	r.BindFlags(flags)
	cfg.BindFlags(flags)

	cmd.Execute()
}

type sink struct {
	start time.Time
}

func (s *sink) Setup() { /* noop */ }

func (s *sink) Reset() {
	s.start = time.Time{}
}

func (s *sink) WaitRequest() (nodeName string, err error) {
	fmt.Println("< req", cfg.NodeName, "at", time.Now())
	return cfg.NodeName, nil
}

var prevs = map[string]proto.Message{}

func (s *sink) Send(op *localnetv1.OpItem) (err error) {
	if s.start.IsZero() {
		s.start = time.Now()
		fmt.Println("< recv at", s.start)
	}

	switch v := op.Op; v.(type) {
	case *localnetv1.OpItem_Set:
		set := op.GetSet()

		var v proto.Message
		switch set.Ref.Set {
		case localnetv1.Set_ServicesSet:
			v = &localnetv1.Service{}
		case localnetv1.Set_EndpointsSet:
			v = &localnetv1.Endpoint{}

		case localnetv1.Set_GlobalEndpointInfos:
			v = &localnetv1.EndpointInfo{}
		case localnetv1.Set_GlobalNodeInfos:
			v = &localnetv1.NodeInfo{}
		case localnetv1.Set_GlobalServiceInfos:
			v = &localnetv1.ServiceInfo{}

		default:
			klog.Info("unknown set: ", set.Ref.Set)
		}

		if v != nil {
			err = proto.Unmarshal(set.Bytes, v)
			if err != nil {
				klog.Info("failed to parse value: ", err)
				v = nil
			}
		}

		refStr := set.Ref.String()
		if prev, ok := prevs[refStr]; ok {
			fmt.Println("-", refStr, "->", prev)
		}
		fmt.Println("+", refStr, "->", v)

		prevs[refStr] = v
	case *localnetv1.OpItem_Delete:
		refStr := op.GetDelete().String()
		prev := prevs[refStr]

		fmt.Println("-", refStr, "->", prev)
		delete(prevs, refStr)

	case *localnetv1.OpItem_Sync:
		fmt.Println("> sync after", time.Since(s.start))
		s.start = time.Time{}
	}

	return
}
