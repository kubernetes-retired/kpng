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

package kube2store

import (
	"github.com/gobwas/glob"
)

func globsFilter(src map[string]string, globsStr []string) (dst map[string]string) {
	if len(globsStr) == 0 {
		return
	}

	globs := make([]glob.Glob, len(globsStr))

	for i, globStr := range globsStr {
		globs[i] = glob.MustCompile(globStr)
	}

	dst = make(map[string]string)

srcLoop:
	for k, v := range src {
		for _, g := range globs {
			if g.Match(k) {
				dst[k] = v
				continue srcLoop
			}
		}
	}

	return
}
