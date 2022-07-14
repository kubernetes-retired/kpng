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

package tlsflags

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"time"
	// "k8s.io/klog/v2"
)

func Bind(flags FlagSet) (f *Flags) {
	f = &Flags{}
	f.Bind(flags, "")
	return
}

type Flags struct {
	KeyFile,
	CertFile,
	CAFile string
}

// FlagSet matches flag.FlagSet and pflag.FlagSet
type FlagSet interface {
	DurationVar(varPtr *time.Duration, name string, value time.Duration, doc string)
	IntVar(varPtr *int, name string, value int, doc string)
	StringVar(varPtr *string, name, value, doc string)
	Uint64Var(varPtr *uint64, name string, value uint64, doc string)
}

func (f *Flags) Bind(flags FlagSet, prefix string) {
	flags.StringVar(&f.KeyFile, prefix+"tls-key", "", "TLS key file")
	flags.StringVar(&f.CertFile, prefix+"tls-crt", "", "TLS certificate file")
	flags.StringVar(&f.CAFile, prefix+"tls-ca", "", "TLS CA certificate file")
}

func (f *Flags) Config() (cfg *tls.Config) {
	if f == nil || f.CAFile == "" && f.KeyFile == "" && f.CertFile == "" {
		return
	}

	cfg = &tls.Config{}

	if f.KeyFile != "" || f.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(f.CertFile, f.KeyFile)
		if err != nil {
			//	klog.Fatal("failed to load TLS key pair: ", err)
		}

		cfg.Certificates = []tls.Certificate{cert}
	}

	if f.CAFile != "" {
		data, err := ioutil.ReadFile(f.CAFile)
		if err != nil {
			//	klog.Fatal("failed to load TLS CA certificate: ", err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			//	klog.Fatal("failed to parse CA certificate")
		}

		cfg.ClientCAs = pool
		cfg.RootCAs = pool
	}

	return
}
