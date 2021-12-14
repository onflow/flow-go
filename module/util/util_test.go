package util_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	realmodule "github.com/onflow/flow-go/module"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAllReady tests that AllReady closes its returned Ready channel only once
// all input ReadyDone instances close their Ready channel.
func TestAllReady(t *testing.T) {
	cases := []int{0, 1, 100}
	for _, n := range cases {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			testAllReady(n, t)
		})
	}
}

// TestAllDone tests that AllDone closes its returned Done channel only once
// all input ReadyDone instances close their Done channel.
func TestAllDone(t *testing.T) {
	cases := []int{0, 1, 100}
	for _, n := range cases {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			testAllDone(n, t)
		})
	}
}

func testAllDone(n int, t *testing.T) {

	components := make([]realmodule.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = new(module.ReadyDoneAware)
		unittest.ReadyDoneify(components[i])
	}

	unittest.AssertClosesBefore(t, util.AllReady(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Ready")
		mock.AssertNotCalled(t, "Done")
	}
}

func testAllReady(n int, t *testing.T) {

	components := make([]realmodule.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = new(module.ReadyDoneAware)
		unittest.ReadyDoneify(components[i])
	}

	unittest.AssertClosesBefore(t, util.AllDone(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Done")
		mock.AssertNotCalled(t, "Ready")
	}
}

func TestMergeChannels(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		channels := make([]<-chan int, 0)
		merged := util.MergeChannels(channels).(<-chan int)
		_, ok := <-merged
		assert.False(t, ok)
	})
	t.Run("empty array", func(t *testing.T) {
		t.Parallel()
		channels := []<-chan int{}
		merged := util.MergeChannels(channels).(<-chan int)
		_, ok := <-merged
		assert.False(t, ok)
	})
	t.Run("nil slice", func(t *testing.T) {
		t.Parallel()
		var channels []<-chan int
		merged := util.MergeChannels(channels).(<-chan int)
		_, ok := <-merged
		assert.False(t, ok)
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			util.MergeChannels(nil)
		})
	})
	t.Run("map", func(t *testing.T) {
		t.Parallel()
		channels := make(map[string]<-chan int)
		assert.Panics(t, func() {
			util.MergeChannels(channels)
		})
	})
	t.Run("string", func(t *testing.T) {
		t.Parallel()
		channels := "abcde"
		assert.Panics(t, func() {
			util.MergeChannels(channels)
		})
	})
	t.Run("array of non-channel", func(t *testing.T) {
		t.Parallel()
		channels := []int{1, 2, 3}
		assert.Panics(t, func() {
			util.MergeChannels(channels)
		})
	})
	t.Run("send channel", func(t *testing.T) {
		t.Parallel()
		channels := []chan<- int{make(chan int), make(chan int)}
		assert.Panics(t, func() {
			util.MergeChannels(channels)
		})
	})
	t.Run("cast returned channel to send channel", func(t *testing.T) {
		t.Parallel()
		channels := []<-chan int{make(<-chan int), make(<-chan int)}
		_, ok := util.MergeChannels(channels).(chan int)
		assert.False(t, ok)
	})
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		channels := []chan int{make(chan int), make(chan int), make(chan int)}
		merged := util.MergeChannels(channels).(<-chan int)
		for i, ch := range channels {
			i := i
			ch := ch
			go func() {
				ch <- i
				close(ch)
			}()
		}
		var elements []int
		for i := range merged {
			elements = append(elements, i)
		}
		assert.ElementsMatch(t, elements, []int{0, 1, 2})
	})
}
