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

package backendcmd

import (
	"github.com/spf13/pflag"

	"sigs.k8s.io/kpng/client/localsink"
)

type Cmd interface {
	BindFlags(*pflag.FlagSet)
	Sink() localsink.Sink
}

var registry []UseCmd

type UseCmd struct {
	Use string
	New func() Cmd
}

func Register(use string, new func() Cmd) {
	registry = append(registry, UseCmd{Use: use, New: new})
}

func Registered() []UseCmd {
	return registry
}
