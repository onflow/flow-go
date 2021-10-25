package component_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const CHANNEL_CLOSE_LATENCY_ALLOWANCE = 20 * time.Millisecond

// IsClosed checks whether a channel is closed
func IsClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

type WorkerState int

const (
	UnknownWorkerState            = iota
	WorkerStartingUp              // worker is starting up
	WorkerStartupShuttingDown     // worker was canceled during startup and is shutting down
	WorkerStartupCanceled         // worker has exited after being canceled during startup
	WorkerStartupEncounteredFatal // worker encountered a fatal error during startup
	WorkerRunning                 // worker has started up and is running normally
	WorkerShuttingDown            // worker was canceled and is shutting down
	WorkerCanceled                // worker has exited after being canceled
	WorkerEncounteredFatal        // worker encountered a fatal error
	WorkerDone                    // worker has shut down after running normally

)

func (s WorkerState) String() string {
	switch s {
	case WorkerStartingUp:
		return "WORKER_STARTING_UP"
	case WorkerStartupShuttingDown:
		return "WORKER_STARTUP_SHUTTING_DOWN"
	case WorkerStartupCanceled:
		return "WORKER_STARTUP_CANCELED"
	case WorkerStartupEncounteredFatal:
		return "WORKER_STARTUP_ENCOUNTERED_FATAL"
	case WorkerRunning:
		return "WORKER_RUNNING"
	case WorkerShuttingDown:
		return "WORKER_SHUTTING_DOWN"
	case WorkerCanceled:
		return "WORKER_CANCELED"
	case WorkerEncounteredFatal:
		return "WORKER_ENCOUNTERED_FATAL"
	case WorkerDone:
		return "WORKER_DONE"
	default:
		return "UNKNOWN"
	}
}

type WorkerStateList []WorkerState

func (wsl WorkerStateList) Contains(ws WorkerState) bool {
	for _, s := range wsl {
		if s == ws {
			return true
		}
	}
	return false
}

type WorkerStateTransition int

const (
	UnknownWorkerStateTransition WorkerStateTransition = iota
	WorkerCheckCtxAndShutdown                          // check context and shutdown if canceled
	WorkerCheckCtxAndExit                              // check context and exit immediately if canceled
	WorkerFinishStartup                                // finish starting up
	WorkerDoWork                                       // do work
	WorkerExit                                         // exit
	WorkerThrowError                                   // throw error
)

func (wst WorkerStateTransition) String() string {
	switch wst {
	case WorkerCheckCtxAndShutdown:
		return "WORKER_CHECK_CTX_AND_SHUTDOWN"
	case WorkerCheckCtxAndExit:
		return "WORKER_CHECK_CTX_AND_EXIT"
	case WorkerFinishStartup:
		return "WORKER_FINISH_STARTUP"
	case WorkerDoWork:
		return "WORKER_DO_WORK"
	case WorkerExit:
		return "WORKER_EXIT"
	case WorkerThrowError:
		return "WORKER_THROW_ERROR"
	default:
		return "UNKNOWN"
	}
}

// WorkerStateTransitions is a map from worker state to valid state transitions
var WorkerStateTransitions = map[WorkerState][]WorkerStateTransition{
	WorkerStartingUp:          {WorkerCheckCtxAndExit, WorkerCheckCtxAndShutdown, WorkerDoWork, WorkerFinishStartup},
	WorkerStartupShuttingDown: {WorkerDoWork, WorkerExit, WorkerThrowError},
	WorkerRunning:             {WorkerCheckCtxAndExit, WorkerCheckCtxAndShutdown, WorkerDoWork, WorkerExit, WorkerThrowError},
	WorkerShuttingDown:        {WorkerDoWork, WorkerExit, WorkerThrowError},
}

// CheckWorkerStateTransition checks the validity of a worker state transition
func CheckWorkerStateTransition(t *rapid.T, id int, start, end WorkerState, transition WorkerStateTransition, canceled bool) {
	if !(func() bool {
		switch start {
		case WorkerStartingUp:
			switch transition {
			case WorkerCheckCtxAndExit:
				return (canceled && end == WorkerStartupCanceled) || (!canceled && end == WorkerStartingUp)
			case WorkerCheckCtxAndShutdown:
				return (canceled && end == WorkerStartupShuttingDown) || (!canceled && end == WorkerStartingUp)
			case WorkerDoWork:
				return end == WorkerStartingUp
			case WorkerFinishStartup:
				return end == WorkerRunning
			}
		case WorkerStartupShuttingDown:
			switch transition {
			case WorkerDoWork:
				return end == WorkerStartupShuttingDown
			case WorkerExit:
				return end == WorkerStartupCanceled
			case WorkerThrowError:
				return end == WorkerStartupEncounteredFatal
			}
		case WorkerRunning:
			switch transition {
			case WorkerCheckCtxAndExit:
				return (canceled && end == WorkerCanceled) || (!canceled && end == WorkerRunning)
			case WorkerCheckCtxAndShutdown:
				return (canceled && end == WorkerShuttingDown) || (!canceled && end == WorkerRunning)
			case WorkerDoWork:
				return end == WorkerRunning
			case WorkerExit:
				return end == WorkerDone
			case WorkerThrowError:
				return end == WorkerEncounteredFatal
			}
		case WorkerShuttingDown:
			switch transition {
			case WorkerDoWork:
				return end == WorkerShuttingDown
			case WorkerExit:
				return end == WorkerCanceled
			case WorkerThrowError:
				return end == WorkerEncounteredFatal
			}
		}

		return false
	}()) {
		require.Fail(t, "invalid worker state transition", "[worker %v] start=%v, canceled=%v, transition=%v, end=%v", id, start, canceled, transition, end)
	}
}

type WSTConsumer func(WorkerStateTransition) WorkerState
type WSTProvider func(WorkerState) WorkerStateTransition

// MakeWorkerTransitionFuncs creates a WorkerStateTransition Consumer / Provider pair.
// The Consumer is called by the worker to notify the test code of the completion of a state transition
// and receive the next state transition instruction.
// The Provider is called by the test code to send the next state transition instruction and get the
// resulting end state.
func MakeWorkerTransitionFuncs() (WSTConsumer, WSTProvider) {
	var started bool
	stateChan := make(chan WorkerState, 1)
	transitionChan := make(chan WorkerStateTransition)

	consumer := func(wst WorkerStateTransition) WorkerState {
		transitionChan <- wst
		return <-stateChan
	}

	provider := func(ws WorkerState) WorkerStateTransition {
		if started {
			stateChan <- ws
		} else {
			started = true
		}

		if _, ok := WorkerStateTransitions[ws]; !ok {
			return UnknownWorkerStateTransition
		}

		return <-transitionChan
	}

	return consumer, provider
}

func ComponentWorker(t *rapid.T, id int, next WSTProvider) component.ComponentWorker {
	unexpectedStateTransition := func(s WorkerState, wst WorkerStateTransition) {
		panic(fmt.Sprintf("[worker %v] unexpected state transition: received %v for state %v", id, wst, s))
	}

	log := func(msg string) {
		t.Logf("[worker %v] %v\n", id, msg)
	}

	return func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc, lookup component.LookupFunc) {
		var state WorkerState
		goto startingUp

	startingUp:
		log("starting up")
		state = WorkerStartingUp
		switch transition := next(state); transition {
		case WorkerCheckCtxAndExit:
			if IsClosed(ctx.Done()) {
				goto startupCanceled
			}
			goto startingUp
		case WorkerCheckCtxAndShutdown:
			if IsClosed(ctx.Done()) {
				goto startupShuttingDown
			}
			goto startingUp
		case WorkerDoWork:
			goto startingUp
		case WorkerFinishStartup:
			ready()
			goto running
		default:
			unexpectedStateTransition(state, transition)
		}

	startupShuttingDown:
		log("startup shutting down")
		state = WorkerStartupShuttingDown
		switch transition := next(state); transition {
		case WorkerDoWork:
			goto startupShuttingDown
		case WorkerExit:
			goto startupCanceled
		case WorkerThrowError:
			goto startupEncounteredFatal
		default:
			unexpectedStateTransition(state, transition)
		}

	startupCanceled:
		log("startup canceled")
		state = WorkerStartupCanceled
		next(state)
		return

	startupEncounteredFatal:
		log("startup encountered fatal")
		state = WorkerStartupEncounteredFatal
		defer next(state)
		ctx.Throw(&WorkerError{id})

	running:
		log("running")
		state = WorkerRunning
		switch transition := next(state); transition {
		case WorkerCheckCtxAndExit:
			if IsClosed(ctx.Done()) {
				goto canceled
			}
			goto running
		case WorkerCheckCtxAndShutdown:
			if IsClosed(ctx.Done()) {
				goto shuttingDown
			}
			goto running
		case WorkerDoWork:
			goto running
		case WorkerExit:
			goto done
		case WorkerThrowError:
			goto encounteredFatal
		default:
			unexpectedStateTransition(state, transition)
		}

	shuttingDown:
		log("shutting down")
		state = WorkerShuttingDown
		switch transition := next(state); transition {
		case WorkerDoWork:
			goto shuttingDown
		case WorkerExit:
			goto canceled
		case WorkerThrowError:
			goto encounteredFatal
		default:
			unexpectedStateTransition(state, transition)
		}

	canceled:
		log("canceled")
		state = WorkerCanceled
		next(state)
		return

	encounteredFatal:
		log("encountered fatal")
		state = WorkerEncounteredFatal
		defer next(state)
		ctx.Throw(&WorkerError{id})

	done:
		log("done")
		state = WorkerDone
		next(state)
	}
}

type WorkerError struct {
	id int
}

func (e *WorkerError) Is(target error) bool {
	if t, ok := target.(*WorkerError); ok {
		return t.id == e.id
	}
	return false
}

func (e *WorkerError) Error() string {
	return fmt.Sprintf("[worker %v] irrecoverable error", e.id)
}

// StartStateTransition returns a pair of functions AddTransition and ExecuteTransitions.
// AddTransition is called to add a state transition step, and then ExecuteTransitions shuffles
// all of the added steps and executes them in a random order.
func StartStateTransition() (func(t func()), func(*rapid.T)) {
	var transitions []func()

	addTransition := func(t func()) {
		transitions = append(transitions, t)
	}

	executeTransitions := func(t *rapid.T) {
		for i := 0; i < len(transitions); i++ {
			j := rapid.IntRange(0, len(transitions)-i-1).Draw(t, "").(int)
			transitions[i], transitions[j+i] = transitions[j+i], transitions[i]
			transitions[i]()
		}
		// TODO: is this simpler?
		// executionOrder := rapid.SliceOfNDistinct(
		// 	rapid.IntRange(0, len(transitions)-1), len(transitions), len(transitions), nil,
		// ).Draw(t, "transition_execution_order").([]int)
		// for _, i := range executionOrder {
		// 	transitions[i]()
		// }
	}

	return addTransition, executeTransitions
}

type StateTransition struct {
	cancel            bool
	workerIDs         []int
	workerTransitions []WorkerStateTransition
}

func (st *StateTransition) String() string {
	return fmt.Sprintf(
		"stateTransition{ cancel=%v, workerIDs=%v, workerTransitions=%v }",
		st.cancel, st.workerIDs, st.workerTransitions,
	)
}

type ComponentManagerMachine struct {
	cm *component.ComponentManager

	cancel                    context.CancelFunc
	workerTransitionConsumers []WSTConsumer

	canceled     bool
	workerErrors error
	workerStates []WorkerState

	resetChannelReadTimeout  func()
	assertClosed             func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{})
	assertNotClosed          func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{})
	assertErrorThrownMatches func(t *rapid.T, err error, msgAndArgs ...interface{})
	assertErrorNotThrown     func(t *rapid.T)

	cancelGenerator     *rapid.Generator
	drawStateTransition func(t *rapid.T) *StateTransition
}

func (c *ComponentManagerMachine) Init(t *rapid.T) {
	numWorkers := rapid.IntRange(0, 5).Draw(t, "num_workers").(int)
	pCancel := rapid.Float64Range(0, 100).Draw(t, "p_cancel").(float64)

	c.cancelGenerator = rapid.Float64Range(0, 100).
		Map(func(n float64) bool {
			return pCancel == 100 || n < pCancel
		})

	c.drawStateTransition = func(t *rapid.T) *StateTransition {
		st := &StateTransition{}

		if !c.canceled {
			st.cancel = c.cancelGenerator.Draw(t, "cancel").(bool)
		}

		for workerId, state := range c.workerStates {
			if allowedTransitions, ok := WorkerStateTransitions[state]; ok {
				label := fmt.Sprintf("worker_transition_%v", workerId)
				st.workerIDs = append(st.workerIDs, workerId)
				st.workerTransitions = append(st.workerTransitions, rapid.SampledFrom(allowedTransitions).Draw(t, label).(WorkerStateTransition))
			}
		}

		return rapid.Just(st).Draw(t, "state_transition").(*StateTransition)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	var channelReadTimeout <-chan struct{}
	var signalerErr error

	c.resetChannelReadTimeout = func() {
		ctx, cancel := context.WithTimeout(context.Background(), CHANNEL_CLOSE_LATENCY_ALLOWANCE)
		_ = cancel
		channelReadTimeout = ctx.Done()
	}

	c.assertClosed = func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{}) {
		select {
		case <-ch:
		default:
			select {
			case <-channelReadTimeout:
				assert.Fail(t, "channel is not closed", msgAndArgs...)
			case <-ch:
			}
		}
	}

	c.assertNotClosed = func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{}) {
		select {
		case <-ch:
			assert.Fail(t, "channel is closed", msgAndArgs...)
		default:
			select {
			case <-ch:
				assert.Fail(t, "channel is closed", msgAndArgs...)
			case <-channelReadTimeout:
			}
		}
	}

	c.assertErrorThrownMatches = func(t *rapid.T, err error, msgAndArgs ...interface{}) {
		if signalerErr == nil {
			select {
			case signalerErr = <-errChan:
			default:
				select {
				case <-channelReadTimeout:
					assert.Fail(t, "error was not thrown")
					return
				case signalerErr = <-errChan:
				}
			}
		}

		assert.ErrorIs(t, err, signalerErr, msgAndArgs...)
	}

	c.assertErrorNotThrown = func(t *rapid.T) {
		if signalerErr == nil {
			select {
			case signalerErr = <-errChan:
			default:
				select {
				case signalerErr = <-errChan:
				case <-channelReadTimeout:
					return
				}
			}
		}

		assert.Fail(t, "error was thrown: %v", signalerErr)
	}

	c.workerTransitionConsumers = make([]WSTConsumer, numWorkers)
	c.workerStates = make([]WorkerState, numWorkers)

	cmb := component.NewComponentManagerBuilder()

	for i := 0; i < numWorkers; i++ {
		wtc, wtp := MakeWorkerTransitionFuncs()
		c.workerTransitionConsumers[i] = wtc
		cmb.AddWorker("main", ComponentWorker(t, i, wtp))
	}

	c.cm = cmb.Build()
	c.cm.Start(signalerCtx)

	for i := 0; i < numWorkers; i++ {
		c.workerStates[i] = WorkerStartingUp
	}
}

func (c *ComponentManagerMachine) ExecuteStateTransition(t *rapid.T) {
	st := c.drawStateTransition(t)

	t.Logf("drew state transition: %v\n", st)

	var errors *multierror.Error

	addTransition, executeTransitionsInRandomOrder := StartStateTransition()

	if st.cancel {
		addTransition(func() {
			t.Log("executing cancel transition\n")
			c.cancel()
			c.canceled = true
			c.resetChannelReadTimeout()
			c.assertClosed(t, c.cm.ShutdownSignal())
		})
	}

	for i, workerId := range st.workerIDs {
		i := i
		workerId := workerId
		addTransition(func() {
			wst := st.workerTransitions[i]
			t.Logf("executing worker %v transition: %v\n", workerId, wst)
			endState := c.workerTransitionConsumers[workerId](wst)
			CheckWorkerStateTransition(t, workerId, c.workerStates[workerId], endState, wst, c.canceled)
			c.workerStates[workerId] = endState

			if (WorkerStateList{WorkerStartupEncounteredFatal, WorkerEncounteredFatal}).Contains(endState) {
				err := &WorkerError{workerId}
				require.NotErrorIs(t, c.workerErrors, err)
				require.NotErrorIs(t, errors, err)
				errors = multierror.Append(errors, err)
				c.canceled = true
				c.resetChannelReadTimeout()
				c.assertClosed(t, c.cm.ShutdownSignal())
			}
		})
	}

	executeTransitionsInRandomOrder(t)

	if c.workerErrors == nil {
		c.workerErrors = errors.ErrorOrNil()
	}

	t.Logf("end state: { canceled=%v, workerErrors=%v, workerStates=%v }\n", c.canceled, c.workerErrors, c.workerStates)
}

func (c *ComponentManagerMachine) Check(t *rapid.T) {
	c.resetChannelReadTimeout()

	if c.canceled {
		c.assertClosed(t, c.cm.ShutdownSignal(), "context is canceled but component manager shutdown signal is not closed")
	}

	allWorkersReady := true
	allWorkersDone := true

	for workerID, state := range c.workerStates {
		if (WorkerStateList{
			WorkerStartingUp,
			WorkerStartupShuttingDown,
			WorkerStartupCanceled,
			WorkerStartupEncounteredFatal,
		}).Contains(state) {
			allWorkersReady = false
			c.assertNotClosed(t, c.cm.Ready(), "worker %v has not finished startup but component manager ready channel is closed", workerID)
		}

		if !(WorkerStateList{
			WorkerStartupCanceled,
			WorkerStartupEncounteredFatal,
			WorkerCanceled,
			WorkerEncounteredFatal,
			WorkerDone,
		}).Contains(state) {
			allWorkersDone = false
			c.assertNotClosed(t, c.cm.Done(), "worker %v has not exited but component manager done channel is closed", workerID)
		}

		if (WorkerStateList{
			WorkerStartupShuttingDown,
			WorkerStartupCanceled,
			WorkerStartupEncounteredFatal,
			WorkerShuttingDown,
			WorkerCanceled,
			WorkerEncounteredFatal,
		}).Contains(state) {
			c.assertClosed(t, c.cm.ShutdownSignal(), "worker %v has been canceled or encountered a fatal error but component manager shutdown signal is not closed", workerID)
		}
	}

	if allWorkersReady {
		c.assertClosed(t, c.cm.Ready(), "all workers are ready but component manager ready channel is not closed")
	}

	if allWorkersDone {
		c.assertClosed(t, c.cm.Done(), "all workers are done but component manager done channel is not closed")
	}

	if c.workerErrors != nil {
		c.assertErrorThrownMatches(t, c.workerErrors, "error received by signaler does not match any of the ones thrown")
		c.assertClosed(t, c.cm.ShutdownSignal(), "fatal error thrown but context is not canceled")
	} else {
		c.assertErrorNotThrown(t)
	}
}

func TestComponentManager(t *testing.T) {
	// skip because this test takes too long
	t.Skip()

	rapid.Check(t, rapid.Run(&ComponentManagerMachine{}))
}
