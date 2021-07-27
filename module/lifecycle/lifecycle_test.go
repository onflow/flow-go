package lifecycle_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	realmodule "github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
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
	var numStarts uint32

	for i := 0; i < 10; i++ {
		go func() {
			suite.lm.OnStart(func() {
				atomic.AddUint32(&numStarts, 1)
			})
		}()
	}

	unittest.RequireCloseBefore(suite.T(), suite.lm.Started(), time.Second, "timed out waiting for startup")
	time.Sleep(100 * time.Millisecond) // wait for potential race conditions to occur

	suite.Assert().EqualValues(1, numStarts)
}

// TestConsecutiveStop tests that calling OnStop multiple times concurrently only
// results in shutdown being performed once
func (suite *LifecycleManagerSuite) TestConsecutiveStop() {
	suite.lm.OnStart()
	unittest.RequireCloseBefore(suite.T(), suite.lm.Started(), time.Second, "timed out waiting for startup")

	var numStops uint32

	for i := 0; i < 10; i++ {
		go func() {
			suite.lm.OnStop(func() {
				atomic.AddUint32(&numStops, 1)
			})
		}()
	}

	unittest.RequireCloseBefore(suite.T(), suite.lm.Stopped(), time.Second, "timed out waiting for shutdown")
	time.Sleep(100 * time.Millisecond) // wait for potential race conditions to occur

	suite.Assert().EqualValues(numStops, 1)
}

// TestStopBeforeStart tests that calling OnStop before OnStart results in startup never
// being performed, and the returned channel never closing.
func (suite *LifecycleManagerSuite) TestStopBeforeStart() {
	suite.lm.OnStop(func() {
		suite.FailNow("shutdown should not occur")
	})

	suite.lm.OnStart(func() {
		suite.FailNow("startup should not occur")
	})

	unittest.RequireCloseBefore(suite.T(), suite.lm.Stopped(), time.Second, "timed out waiting for shutdown")
	unittest.RequireNotClosed(suite.T(), suite.lm.Started(), "Started channel should never close")
}

// TestStopAfterStart tests that if OnStop is called right after OnStart, shutdown will
// only be performed after startup has finished.
func (suite *LifecycleManagerSuite) TestStopAfterStart() {
	var started uint32 = 0

	suite.lm.OnStart(func() {
		// simulate startup processing
		time.Sleep(3 * time.Second)
		atomic.StoreUint32(&started, 1)
	})

	suite.lm.OnStop(func() {
		suite.Assert().EqualValues(atomic.LoadUint32(&started), 1)
	})

	unittest.RequireCloseBefore(suite.T(), suite.lm.Stopped(), 5*time.Second, "timed out waiting for shutdown")
	unittest.RequireClosed(suite.T(), suite.lm.Started(), "Started channel should be closed")
}

// TestHappyPath tests a normal start-stop lifecycle.
func (suite *LifecycleManagerSuite) TestHappyPath() {
	suite.lm.OnStart()

	unittest.AssertClosesBefore(suite.T(), suite.lm.Started(), time.Second)
	unittest.RequireNotClosed(suite.T(), suite.lm.Stopped(), "Stopped channel should not close")

	suite.lm.OnStop()

	unittest.AssertClosesBefore(suite.T(), suite.lm.Stopped(), time.Second)
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
