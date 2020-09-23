package lifecycle_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/lifecycle"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type readydone struct{}

func (readydone) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(rand.Intn(10)) * time.Microsecond)
		close(ready)
	}()
	return ready
}

func (readydone) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(rand.Intn(10)) * time.Microsecond)
		close(done)
	}()
	return done
}

func TestAllReady(t *testing.T) {
	n := 100

	components := make([]module.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = readydone{}
	}

	unittest.AssertClosesBefore(t, lifecycle.AllReady(components...), time.Second)
}

func TestAllDone(t *testing.T) {
	n := 100

	components := make([]module.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = readydone{}
	}

	unittest.AssertClosesBefore(t, lifecycle.AllDone(components...), time.Second)
}
