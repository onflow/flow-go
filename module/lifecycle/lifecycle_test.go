package lifecycle_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	realmodule "github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
)

type LifecycleManagerSuite struct {
	suite.Suite
	lm *lifecycle.LifecycleManager
}

func (suite *LifecycleManagerSuite) SetupTest() {
	suite.lm = lifecycle.NewLifecycleManager()
}

// TestConsecutiveStart tests that calling OnStart multiple times concurrently only
// results in startup being performed once
func (suite *LifecycleManagerSuite) TestConsecutiveStart() {
	cases := []int{1, 2, 10}
	for _, n := range cases {
		suite.T().Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			suite.testConsecutiveStart(suite.lm, n)
		})
	}
}

func (suite *LifecycleManagerSuite) testConsecutiveStart(lm *lifecycle.LifecycleManager, n int) {
	var wg sync.WaitGroup
	var numStarts uint32 = 0

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			lm.OnStart(func() {
				atomic.AddUint32(&numStarts, 1)
				wg.Done()
			})
		}()
	}

	wg.Wait()

	suite.Assert().Equal(numStarts, 1)
}

func TestLifecycleManager(t *testing.T) {
	suite.Run(t, new(LifecycleManagerSuite))
}

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

	unittest.AssertClosesBefore(t, lifecycle.AllReady(components...), time.Second)

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

	unittest.AssertClosesBefore(t, lifecycle.AllDone(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Done")
		mock.AssertNotCalled(t, "Ready")
	}
}
