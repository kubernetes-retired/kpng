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
	"log"
	"os"
	"text/template"
)

type KnpgDameonSetData struct {
	Namespace          string
	ServiceAccountName string
	ImagePullPolicy    string
	IsEbpfBackend      bool
	KpngImgage         string
	Backend            string
	E2eBackendArgs     string
	ConfigMapName      string
}

func main() {
	if len(os.Args) < 3 {
		os.Exit(1)
	}
	templPath := os.Args[1]
	yamlDestPath := os.Args[2]
	if len(templPath) == 0 || len(yamlDestPath) == 0 {
		os.Exit(1)
	}
	tmpl, err := template.ParseFiles(templPath)
	if err != nil {
		log.Fatalf("unable to open template %v", err)
	}
	data := KnpgDameonSetData{
		Namespace:          os.Getenv("namespace"),
		ServiceAccountName: os.Getenv("service_account_name"),
		ImagePullPolicy:    os.Getenv("image_pull_policy"),
		IsEbpfBackend:      os.Getenv("backend") == "ebpf",
		KpngImgage:         os.Getenv("kpng_image"),
		Backend:            os.Getenv("backend"),
		E2eBackendArgs:     os.Getenv("e2e_backend_args"),
		ConfigMapName:      os.Getenv("config_map_name"),
	}

	f, err := os.Create(yamlDestPath)
	if err != nil {
		os.Exit(1)
	}
	err = tmpl.Execute(f, data)

}
