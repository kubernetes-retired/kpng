package store2file

import (
	"context"
	"os"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"

	"k8s.io/klog"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	proxystore "sigs.k8s.io/kpng/server/pkg/proxystore"
)

type Config struct {
	FilePath string
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&c.FilePath, "output", "o", "global-state.yaml", "Output file for the global state")
}

type Job struct {
	Store  *proxystore.Store
	Config *Config
}

func (j *Job) Run(ctx context.Context) (err error) {
	var (
		rev    uint64
		closed = false
	)

	for !closed {
		state := GlobalState{}
		ok := false

		rev, closed = j.Store.View(rev, func(tx *proxystore.Tx) {
			if !tx.AllSynced() {
				return
			}

			tx.Each(proxystore.Nodes, func(kv *proxystore.KV) bool {
				state.Nodes = append(state.Nodes, kv.Node.Node)
				return true
			})

			tx.Each(proxystore.Services, func(kv *proxystore.KV) bool {
				sae := ServiceAndEndpoints{
					Service:      kv.Service.Service,
					TopologyKeys: kv.Service.TopologyKeys,
				}

				tx.EachEndpointOfService(kv.Namespace, kv.Name, func(ep *localnetv1.EndpointInfo) {
					sae.Endpoints = append(sae.Endpoints, ep)
				})

				state.Services = append(state.Services, sae)

				return true
			})

			ok = true
		})

		if !ok {
			continue
		}

		// write the output
		var out *os.File
		out, err = os.Create(j.Config.FilePath)
		if err != nil {
			return
		}

		err = yaml.NewEncoder(out).Encode(state)
		out.Close()
		if err != nil {
			return
		}

		klog.Info("wrote global state")
	}

	return
}

type GlobalState struct {
	Nodes    []*localnetv1.Node
	Services []ServiceAndEndpoints
}

type ServiceAndEndpoints struct {
	Service      *localnetv1.Service
	TopologyKeys []string
	Endpoints    []*localnetv1.EndpointInfo
}
