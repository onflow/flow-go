package module

import (
	"fmt"
	"testing"
	"time"

	realmodule "github.com/onflow/flow-go/module"
	module "github.com/onflow/flow-go/module/mock"
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

	unittest.AssertClosesBefore(t, AllReady(components...), time.Second)

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

	unittest.AssertClosesBefore(t, module.AllDone(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Done")
		mock.AssertNotCalled(t, "Ready")
	}
}
