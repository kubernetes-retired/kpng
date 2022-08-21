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

package fullstatepipe

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"sigs.k8s.io/kpng/api/localnetv1"
	//"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

func ExampleSequence() {
	go failAfter1Sec()

	pipe := New(Sequence,
		delayCallback("s1", 3*time.Millisecond),
		delayCallback("s2", 2*time.Millisecond),
		delayCallback("s3", 1*time.Millisecond),
	)

	pipe.Callback(singleServiceCh("my-service"))

	out.print()

	// Output:
	// s1 got service my-service
	// s1 finished
	// s2 got service my-service
	// s2 finished
	// s3 got service my-service
	// s3 finished
}

func ExampleParallel() {
	go failAfter1Sec()

	pipe := New(Parallel,
		delayCallback("s1", 8*time.Millisecond),
		delayCallback("s2", 3*time.Millisecond),
		delayCallback("s3", 1*time.Millisecond),
	)

	pipe.Callback(singleServiceCh("my-service"))

	out.print()

	// Output:
	// s3 got service my-service
	// s3 finished
	// s2 got service my-service
	// s2 finished
	// s1 got service my-service
	// s1 finished
}

func ExampleParallelSendSequenceClose() {
	go failAfter1Sec()

	pipe := New(ParallelSendSequenceClose,
		delayCallback("s1", 8*time.Millisecond),
		delayCallback("s2", 3*time.Millisecond),
		delayCallback("s3", 1*time.Millisecond),
	)

	pipe.Callback(singleServiceCh("my-service"))

	out.print()

	// Output:
	// s3 got service my-service
	// s2 got service my-service
	// s1 got service my-service
	// s1 finished
	// s2 finished
	// s3 finished
}

func failAfter1Sec() {
	time.Sleep(time.Second)
	panic("example timed out")
}

// ------------------------------------------------------------------------
// required for stable outputs (and fast tests)
//

type syncBuf struct {
	sync.Mutex
	bytes.Buffer
}

var out = &syncBuf{}

func (out *syncBuf) Write(ba []byte) (n int, err error) {
	out.Lock()
	defer out.Unlock()
	return out.Buffer.Write(ba)
}

func (out *syncBuf) print() {
	io.Copy(os.Stdout, out)
}

func delayCallback(name string, delay time.Duration) fullstate.Callback {
	return func(ch <-chan *fullstate.ServiceEndpoints) {
		for item := range ch {
			time.Sleep(delay)
			fmt.Fprintln(out, name, "got service", item.Service.Name)
		}

		time.Sleep(delay)
		fmt.Fprintln(out, name, "finished")
	}
}

func singleServiceCh(svcName string) (ch chan *fullstate.ServiceEndpoints) {
	ch = make(chan *fullstate.ServiceEndpoints, 1)
	ch <- &fullstate.ServiceEndpoints{Service: &localnetv1.Service{Name: svcName}}
	close(ch)
	return
}
