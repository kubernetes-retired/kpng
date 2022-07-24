package fullstatepipe

import (
	"fmt"
	"sync"

	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

type Strategy int

const (
	// Sequence calls to each pipe stage in sequence. Implies storing the state in a buffer.
	Sequence = iota
	// Parallel calls each pipe stage in parallel. No buffering required, but
	// the stages are not really stages anymore.
	Parallel
	// ParallelSendSequenceClose calls each pipe entry in parallel but closes
	// the channel of a stage only after the previous has finished. No
	// buffering required but still a meaningful sequencing, especially when
	// using the diffstore.
	ParallelSendSequenceClose
)

type Pipe struct {
	strategy Strategy
	stages   []fullstate.Callback
	buffer   []*client.ServiceEndpoints
}

func New(strategy Strategy, stages ...fullstate.Callback) *Pipe {
	return &Pipe{
		strategy: strategy,
		stages:   stages,
	}
}

func (pipe *Pipe) Callback(ch <-chan *client.ServiceEndpoints) {
	switch pipe.strategy {
	case Sequence:
		if pipe.buffer == nil {
			pipe.buffer = make([]*client.ServiceEndpoints, 0)
		}

		buf := pipe.buffer

		for item := range ch {
			buf = append(buf, item)
		}

		for _, stage := range pipe.stages {
			myCh := make(chan *client.ServiceEndpoints, 1)
			go func() {
				for _, item := range buf {
					myCh <- item
				}
				close(myCh)
			}()

			stage(myCh)
		}

		pipe.buffer = buf[:0]

	case Parallel:
		channels := make([]chan *client.ServiceEndpoints, len(pipe.stages))

		wg := new(sync.WaitGroup)
		wg.Add(len(pipe.stages))

		for idx, stage := range pipe.stages {
			childCh := make(chan *client.ServiceEndpoints, 2)
			channels[idx] = childCh

			stage := stage
			go func() {
				defer wg.Done()
				stage(childCh)
			}()
		}

		for item := range ch {
			for _, childCh := range channels {
				childCh <- item
			}
		}

		for _, childCh := range channels {
			close(childCh)
		}

		wg.Wait()

	case ParallelSendSequenceClose:
		channels := make([]chan *client.ServiceEndpoints, len(pipe.stages))
		waitGroups := make([]*sync.WaitGroup, len(pipe.stages))

		for idx, stage := range pipe.stages {
			childCh := make(chan *client.ServiceEndpoints, 2)
			channels[idx] = childCh

			wg := new(sync.WaitGroup)
			waitGroups[idx] = wg

			stage := stage

			wg.Add(1)
			go func() {
				defer wg.Done()
				stage(childCh)
			}()
		}

		for item := range ch {
			for _, childCh := range channels {
				childCh <- item
			}
		}

		for idx, childCh := range channels {
			close(childCh)
			waitGroups[idx].Wait()
		}

	default:
		panic(fmt.Errorf("unknown strategy: %d", pipe.strategy))
	}
}
