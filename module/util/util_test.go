package util_test

import (
	"context"
	"errors"
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

func TestWaitClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("channel closed returns nil", func(t *testing.T) {
		finished := make(chan struct{})
		ch := make(chan struct{})
		go func() {
			err := util.WaitClosed(ctx, ch)
			assert.NoError(t, err)
			close(finished)
		}()
		close(ch)

		select {
		case <-finished:
		case <-time.After(100 * time.Millisecond):
			t.Error("timed out")
		}
	})

	t.Run("context cancelled returns error", func(t *testing.T) {
		testCtx, testCancel := context.WithCancel(ctx)
		finished := make(chan struct{})
		ch := make(chan struct{})
		go func() {
			err := util.WaitClosed(testCtx, ch)
			assert.ErrorIs(t, err, context.Canceled)
			close(finished)
		}()
		testCancel()

		select {
		case <-finished:
		case <-time.After(100 * time.Millisecond):
			t.Error("timed out")
		}
	})

	t.Run("both conditions triggered returns nil", func(t *testing.T) {
		// both conditions are met when WaitClosed is called. Since one is randomly selected,
		// there is a 99.9% probability that each condition will be picked first at least once
		// during this test.
		for i := 0; i < 10; i++ {
			testCtx, testCancel := context.WithCancel(ctx)
			finished := make(chan struct{})
			ch := make(chan struct{})
			close(ch)
			testCancel()

			go func() {
				err := util.WaitClosed(testCtx, ch)
				assert.NoError(t, err)
				close(finished)
			}()

			select {
			case <-finished:
			case <-time.After(100 * time.Millisecond):
				t.Error("timed out")
			}
		}
	})
}

func TestCheckClosed(t *testing.T) {
	done := make(chan struct{})
	assert.False(t, util.CheckClosed(done))
	close(done)
	assert.True(t, util.CheckClosed(done))
}

func TestWaitError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testErr := errors.New("test error channel")
	t.Run("error received returns error", func(t *testing.T) {
		finished := make(chan struct{})
		ch := make(chan error)

		go func() {
			err := util.WaitError(ch, ctx.Done())
			assert.ErrorIs(t, err, testErr)
			close(finished)
		}()
		ch <- testErr

		select {
		case <-finished:
		case <-time.After(100 * time.Millisecond):
			t.Error("timed out")
		}
	})

	t.Run("context cancelled returns error", func(t *testing.T) {
		testCtx, testCancel := context.WithCancel(ctx)
		finished := make(chan struct{})
		ch := make(chan error)
		go func() {
			err := util.WaitError(ch, testCtx.Done())
			assert.NoError(t, err)
			close(finished)
		}()
		testCancel()

		select {
		case <-finished:
		case <-time.After(100 * time.Millisecond):
			t.Error("timed out")
		}
	})

	t.Run("both conditions triggered returns error", func(t *testing.T) {
		// both conditions are met when WaitError is called. Since one is randomly selected,
		// there is a 99.9% probability that each condition will be picked first at least once
		// during this test.
		for i := 0; i < 10; i++ {
			finished := make(chan struct{})
			ch := make(chan error, 1) // buffered so we can add before starting
			done := make(chan struct{})

			ch <- testErr
			close(done)

			go func() {
				err := util.WaitError(ch, done)
				assert.ErrorIs(t, err, testErr)
				close(finished)
			}()

			select {
			case <-finished:
			case <-time.After(100 * time.Millisecond):
				t.Error("timed out")
			}
		}
	})
}
