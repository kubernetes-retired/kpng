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
	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/cmd/kpng/storecmds"
	"sigs.k8s.io/kpng/server/jobs/api2local"
)

func local2sinkCmd() *cobra.Command {
	// local to * command
	cmd := &cobra.Command{
		Use:   "local",
		Short: "watch kpng API's local state",
	}

	flags := cmd.PersistentFlags()

	job := api2local.New(nil)
	job.BindFlags(flags)

	cmd.AddCommand(storecmds.LocalCmds(func(sink localsink.Sink) (err error) {
		ctx := setupGlobal()
		job.Sink = sink
		job.Run(ctx)
		return
	})...)

	return cmd
}
