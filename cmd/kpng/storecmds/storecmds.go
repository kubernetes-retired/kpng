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

package storecmds

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"

	"sigs.k8s.io/kpng/server/jobs/store2api"
	"sigs.k8s.io/kpng/server/jobs/store2file"
	"sigs.k8s.io/kpng/server/jobs/store2localdiff"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
)

type SetupFunc func() (ctx context.Context, store *proxystore.Store, err error)

func Commands(setup SetupFunc) []*cobra.Command {
	return []*cobra.Command{
		setup.ToAPICmd(),
		setup.ToFileCmd(),
		setup.ToLocalCmd(),
	}
}

func (c SetupFunc) ToAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "to-api",
	}

	cfg := &store2api.Config{}
	cfg.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) (err error) {
		ctx, store, err := c()
		if err != nil {
			return
		}

		j := &store2api.Job{
			Store:  store,
			Config: cfg,
		}
		return j.Run(ctx)
	}

	return cmd
}

func (c SetupFunc) ToFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "to-file",
		Short: "dump global state to a yaml db file",
	}

	cfg := &store2file.Config{}
	cfg.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) (err error) {
		ctx, store, err := c()
		if err != nil {
			return
		}

		j := &store2file.Job{
			Store:  store,
			Config: cfg,
		}
		return j.Run(ctx)
	}

	return cmd
}

// ToLocalCmd sends the incoming events to a local backend, such as IPVS or IPTABLES or NFT.
// This gives users an out of the box KPNG implementation.
func (c SetupFunc) ToLocalCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use: "to-local",
	}

	var ctx context.Context
	job := &store2localdiff.Job{}

	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) (err error) {
		ctx, job.Store, err = c()
		return
	}

	cmd.AddCommand(LocalCmds(func(sink localsink.Sink) error {
		job.Sink = sink
		return job.Run(ctx)
	})...)

	return
}

func LocalCmds(run func(sink localsink.Sink) error) (cmds []*cobra.Command) {
	// sink backends
	for _, useCmd := range backendcmd.Registered() {
		backend := useCmd.New()

		cmd := &cobra.Command{
			Use: useCmd.Use,
			RunE: func(_ *cobra.Command, _ []string) error {
				return run(backend.Sink())
			},
		}

		backend.BindFlags(cmd.Flags())

		cmds = append(cmds, cmd)
	}

	return
}

func unimplemented(_ *cobra.Command, _ []string) error {
	return errors.New("not implemented")
}
