/*
Copyright 2014 The Kubernetes Authors.

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

package util

import (
	"bytes"
	"fmt"
)

type LineBuffer struct {
	b bytes.Buffer
}

// Write takes a list of arguments, each a string or []string, joins all the
// individual strings with spaces, terminates with newline, and writes to buf.
// Any other argument type will panic.
func (buf *LineBuffer) Write(args ...interface{}) {
	for i, arg := range args {
		if i > 0 {
			buf.b.WriteByte(' ')
		}
		switch x := arg.(type) {
		case string:
			buf.b.WriteString(x)
		case []string:
			for j, s := range x {
				if j > 0 {
					buf.b.WriteByte(' ')
				}
				buf.b.WriteString(s)
			}
		default:
			panic(fmt.Sprintf("unknown argument type: %T", x))
		}
	}
	buf.b.WriteByte('\n')
}

// WriteBytes writes bytes to buffer, and terminates with newline.
func (buf *LineBuffer) WriteBytes(bytes []byte) {
	buf.b.Write(bytes)
	buf.b.WriteByte('\n')
}

func (buf *LineBuffer) Reset() {
	buf.b.Reset()
}

func (buf *LineBuffer) Bytes() []byte {
	return buf.b.Bytes()
}
