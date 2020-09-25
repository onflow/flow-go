package lifecycle_test

import (
	"testing"
	"time"

	realmodule "github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAllReady(t *testing.T) {
	n := 100

	components := make([]realmodule.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = new(module.ReadyDoneAware)
		unittest.ReadyDoneify(components[i])
	}

	unittest.AssertClosesBefore(t, lifecycle.AllReady(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Ready")
		mock.AssertNotCalled(t, "Done")
	}
}

func TestAllDone(t *testing.T) {
	n := 100

	components := make([]realmodule.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = new(module.ReadyDoneAware)
		unittest.ReadyDoneify(components[i])
	}

	unittest.AssertClosesBefore(t, lifecycle.AllDone(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Done")
		mock.AssertNotCalled(t, "Ready")
	}
}
