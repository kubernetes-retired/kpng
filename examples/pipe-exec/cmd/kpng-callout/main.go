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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client"
)

type env struct {
	prog string
}

func main() {
	env := env{
		prog: os.Getenv("CALLOUT"),
	}
	if env.prog == "" {
		env.prog = "/bin/cat"
	}
	client.Run(env.callout)
}

func (e *env) callout(items []*fullstate.ServiceEndpoints) {
	cmd := exec.Command(e.prog)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	enc := json.NewEncoder(stdin)
	go func() {
		defer stdin.Close()
		for _, item := range items {
			_ = enc.Encode(item)
		}
	}()
	if out, err := cmd.Output(); err == nil {
		fmt.Println(string(out))
	}
}
