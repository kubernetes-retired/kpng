/*
Copyright 2023 The Kubernetes Authors.
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
	"fmt"
	"github.com/spf13/cobra"
)

var (
	// RELEASE returns the release version
	RELEASE = "UNKNOWN"
	// REPO returns the git repository URL
	REPO = "UNKNOWN"
	// COMMIT returns the short sha from git
	COMMIT = "UNKNOWN"
)

// versionCmd is responsible for printing relevant information regarding KPNG's version
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the version and quit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(kpngVersion())
		},
	}
}

// kpngVersion returns information about the version.
func kpngVersion() string {
	return fmt.Sprintf(`-------------------------------------------------------------------------------
Kubernetes Proxy NG
  Commit:        %v
  Release:       %v
  Repository:    %v
-------------------------------------------------------------------------------
`, COMMIT, RELEASE, REPO)
}
