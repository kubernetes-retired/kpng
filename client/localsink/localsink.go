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

package localsink

import (
	"os"

	"github.com/spf13/pflag"

	"sigs.k8s.io/kpng/api/localv1"
)

type Sink interface {
	// Setup is called once, when the job starts
	Setup()

	// WaitRequest waits for the next diff request, returning the requested node name. If an error is returned, it will cancel the job.
	WaitRequest() (nodeName string, err error)

	// Reset the state of the Sink (ie: when the client is disconnected and reconnects)
	Reset()

	localv1.OpSink
}

type Config struct {
	NodeName string
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&c.NodeName, "node-name", func() string {
		s, _ := os.Hostname()
		return s
	}(), "Node name override")
}

func (c *Config) WaitRequest() (nodeName string, err error) {
	return c.NodeName, nil
}
