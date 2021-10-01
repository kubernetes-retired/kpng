package storecmds

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	ipvs2 "sigs.k8s.io/kpng/backends/ipvs"
	ipvssink2 "sigs.k8s.io/kpng/backends/ipvs-as-sink"
	nft2 "sigs.k8s.io/kpng/backends/nft"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	store2api2 "sigs.k8s.io/kpng/server/jobs/store2api"
	store2file2 "sigs.k8s.io/kpng/server/jobs/store2file"
	store2localdiff2 "sigs.k8s.io/kpng/server/jobs/store2localdiff"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
)

type SetupFunc func() (ctx context.Context, store *proxystore2.Store, err error)

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

	cfg := &store2api2.Config{}
	cfg.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) (err error) {
		ctx, store, err := c()
		if err != nil {
			return
		}

		j := &store2api2.Job{
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

	cfg := &store2file2.Config{}
	cfg.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) (err error) {
		ctx, store, err := c()
		if err != nil {
			return
		}

		j := &store2file2.Job{
			Store:  store,
			Config: cfg,
		}
		return j.Run(ctx)
	}

	return cmd
}

func (c SetupFunc) ToLocalCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use: "to-local",
	}

	var ctx context.Context
	job := &store2localdiff2.Job{}

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
	// classic backends
	cfg := &localsink.Config{}
	sink := fullstate.New(cfg)

	for _, cmd := range BackendCmds(sink, run) {
		cfg.BindFlags(cmd.Flags())
		cmds = append(cmds, cmd)
	}

	// sink backends
	ipvsBackend := ipvssink2.New()

	cmd := &cobra.Command{
		Use: "to-ipvs2",
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(ipvsBackend.Sink())
		},
	}

	ipvsBackend.BindFlags(cmd.Flags())

	cmds = append(cmds, cmd)

	return
}

func BackendCmds(sink *fullstate.Sink, run func(sink localsink.Sink) error) []*cobra.Command {
	return []*cobra.Command{
		//iptablesCommand(sink, run),
		ipvsCommand(sink, run),
		nftCommand(sink, run),
	}
}

func unimplemented(_ *cobra.Command, _ []string) error {
	return errors.New("not implemented")
}

func nftCommand(sink *fullstate.Sink, run func(sink localsink.Sink) error) *cobra.Command {
	cmd := &cobra.Command{
		Use: "to-nft",
	}

	nft2.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		nft2.PreRun()
		sink.Callback = nft2.Callback
		return run(sink)
	}

	return cmd
}

func ipvsCommand(sink *fullstate.Sink, run func(sink localsink.Sink) error) *cobra.Command {
	cmd := &cobra.Command{
		Use: "to-ipvs",
	}

	ipvs2.BindFlags(cmd.Flags())

	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		ipvs2.PreRun()
		sink.Callback = ipvs2.Callback
		return run(sink)
	}

	return cmd
}

// moved to incubating (too many dependencies)
//func iptablesCommand(sink *fullstate.Sink, run func(sink localsink.Sink) error) *cobra.Command {
//	iptablesBackend := iptables2.New()
//	cmd := &cobra.Command{
//		Use: "to-iptables",
//		RunE: func(_ *cobra.Command, _ []string) error {
//			return run(iptablesBackend.Sink())
//		},
//	}
//	iptablesBackend.BindFlags(cmd.Flags())
//	return cmd
//}
