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

	"k8s.io/klog/v2"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"

	"sigs.k8s.io/kpng/server/jobs/store2api"
	"sigs.k8s.io/kpng/server/jobs/store2file"
	"sigs.k8s.io/kpng/server/jobs/store2localdiff"
	"sigs.k8s.io/kpng/server/proxystore"
)

// consumerJobCmd returns a *cobra.Command which sets up the store producer job, starts it
// and then kicks off the store consumer job.
type consumerJobCmd func(ctx context.Context, store *proxystore.Store, storeProducerJobSetup func() (err error), storeProducerJobRun func()) *cobra.Command

// ToAPICmd builds a command that reads from the APIServer and sends data down to the store.
func ToAPICmd(ctx context.Context, store *proxystore.Store, storeProducerJobSetup func() (err error), storeProducerJobRun func()) *cobra.Command {
	cmd := &cobra.Command{
		Use: "to-api",
	}

	cfg := &store2api.Config{}
	cfg.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) (err error) {
		if storeProducerJobSetup != nil {
			if err := storeProducerJobSetup(); err != nil {
				return err
			}
		}
		go storeProducerJobRun()
		j := &store2api.Job{
			Store:  store,
			Config: cfg,
		}
		return j.Run(ctx)
	}

	return cmd
}

// ToFileCmd builds a command that reads from the APIServer and sends data down to a file.
func ToFileCmd(ctx context.Context, store *proxystore.Store, storeProducerJobSetup func() (err error), storeProducerJobRun func()) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "to-file",
		Short: "dump globalv1 state to a yaml db file",
	}

	cfg := &store2file.Config{}
	cfg.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) (err error) {
		if storeProducerJobSetup != nil {
			if err := storeProducerJobSetup(); err != nil {
				return err
			}
		}
		go storeProducerJobRun()

		j := &store2file.Job{
			Store:  store,
			Config: cfg,
		}
		return j.Run(ctx)
	}

	return cmd
}

// ToLocalCmd reads from the store, and sends these down to a local backend, which we refer to as a Sink.
// See the LocalCmds implementation to understand how we use reflection to load up the individual backends
// such that their command line options are dynamically accepted here.
func ToLocalCmd(ctx context.Context, store *proxystore.Store, storeProducerJobSetup func() (err error), storeProducerJobRun func()) (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use: "to-local",
	}

	job := &store2localdiff.Job{}

	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) (err error) {
		job.Store = store
		return
	}

	cmd.AddCommand(LocalCmds(func(sink localsink.Sink) error {
		if storeProducerJobSetup != nil {
			if err := storeProducerJobSetup(); err != nil {
				return err
			}
		}
		go storeProducerJobRun()

		job.Sink = sink
		return job.Run(ctx)
	})...)

	return
}

// LocalCmds uses "reflection", i.e. it depends on the hot-loading of backends when
// the imports are called.  the "Registered" function then adds the backends one at a time.
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
		klog.Infof("Appending discovered command %v", cmd.Name())
		cmds = append(cmds, cmd)
	}

	return
}

func unimplemented(_ *cobra.Command, _ []string) error {
	return errors.New("not implemented")
}
