package component

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/module/irrecoverable"
)

const CHANNEL_CLOSE_LATENCY_ALLOWANCE = 20 * time.Millisecond

func isClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

type componentState int

const (
	unknownComponentState componentState = iota
	componentIdle
	componentStarted
	componentStartingUp
	componentStartupFailed
	componentStartupCanceled
	componentStartupFinished
	componentReadyEncounteredFatal
	componentReadyCanceled
	componentReadyShuttingDown
	componentRunning
	componentShuttingDown
	componentDone
	componentEncounteredFatal
)

func (s componentState) String() string {
	switch s {
	case componentIdle:
		return "component-idle"
	case componentStarted:
		return "component-started"
	case componentStartingUp:
		return "component-starting-up"
	case componentStartupFailed:
		return "component-startup-failed"
	case componentStartupCanceled:
		return "component-startup-canceled"
	case componentStartupFinished:
		return "component-startup-finished"
	case componentRunning:
		return "component-running"
	case componentShuttingDown:
		return "component-shutting-down"
	case componentDone:
		return "component-done"
	case componentEncounteredFatal:
		return "component-encountered-fatal"
	case componentReadyShuttingDown:
		return "component-ready-shutting-down"
	case componentReadyCanceled:
		return "component-ready-canceled"
	case componentReadyEncounteredFatal:
		return "component-ready-encountered-fatal"
	default:
		return "unknown"
	}
}

type componentStateList []componentState

func (csl componentStateList) contains(cs componentState) bool {
	for _, s := range csl {
		if s == cs {
			return true
		}
	}
	return false
}

type componentStateTransition int

const (
	unknownComponentStateTransition componentStateTransition = iota
	componentCheckCtx
	componentCheckCtxWithCleanup
	componentFailStartup
	componentFinishStartup
	componentBecomeReady
	componentThrowErr
	componentFinishShutdown
)

func (st componentStateTransition) String() string {
	switch st {
	case componentCheckCtx:
		return "component-check-ctx"
	case componentCheckCtxWithCleanup:
		return "component-check-ctx-with-cleanup"
	case componentFailStartup:
		return "component-fail-startup"
	case componentFinishStartup:
		return "component-finish-startup"
	case componentBecomeReady:
		return "component-become-ready"
	case componentThrowErr:
		return "component-throw-err"
	case componentFinishShutdown:
		return "component-finish-shutdown"
	default:
		return "unknown"
	}
}

type cstConsumer func(componentStateTransition) componentState
type cstProvider func(componentState) componentStateTransition

type component struct {
	t     *rapid.T
	id    int
	next  cstProvider
	ready chan struct{}
	done  chan struct{}
}

func newComponent(t *rapid.T, id int, next cstProvider) *component {
	return &component{t, id, next, make(chan struct{}), make(chan struct{})}
}

func (c *component) unexpectedStateTransition(s componentState, cst componentStateTransition) {
	assert.Fail(c.t, "unexpected state transition", "[component %v] received %v for state %v", c.id, cst, s)
	runtime.Goexit()
}

func (c *component) Start(ctx irrecoverable.SignalerContext) (err error) {
	var state componentState
	defer func() {
		if err != nil {
			close(c.done)
			c.next(state)
		}
	}()
	goto started

started:
	c.t.Logf("[component %v] started\n", c.id)
	state = componentStarted
	switch transition := c.next(state); transition {
	case componentCheckCtx:
		if isClosed(ctx.Done()) {
			goto startupCanceled
		}
		goto startingUp
	default:
		c.unexpectedStateTransition(state, transition)
	}

startingUp:
	c.t.Logf("[component %v] starting up\n", c.id)
	state = componentStartingUp
	switch transition := c.next(state); transition {
	case componentFailStartup:
		goto startupFailed
	case componentBecomeReady:
		fallthrough
	case componentFinishStartup:
		go func() {
			defer func() {
				close(c.done)
				c.next(state)
			}()

			if transition == componentBecomeReady {
				close(c.ready)
				goto running
			}
			goto startupFinished

		startupFinished:
			c.t.Logf("[component %v] startup finished\n", c.id)
			state = componentStartupFinished
			switch transition := c.next(state); transition {
			case componentBecomeReady:
				close(c.ready)
				goto running
			case componentThrowErr:
				goto readyEncounteredFatal
			case componentCheckCtxWithCleanup:
				if isClosed(ctx.Done()) {
					goto readyShuttingDown
				}
				goto startupFinished
			case componentCheckCtx:
				if isClosed(ctx.Done()) {
					goto readyCanceled
				}
				goto startupFinished
			default:
				c.unexpectedStateTransition(state, transition)
			}

		running:
			c.t.Logf("[component %v] running\n", c.id)
			state = componentRunning
			switch transition := c.next(state); transition {
			case componentThrowErr:
				goto encounteredFatal
			case componentCheckCtxWithCleanup:
				if isClosed(ctx.Done()) {
					goto shuttingDown
				}
				goto running
			case componentCheckCtx:
				if isClosed(ctx.Done()) {
					goto done
				}
				goto running
			default:
				c.unexpectedStateTransition(state, transition)
			}

		readyShuttingDown:
			c.t.Logf("[component %v] ready shutting down\n", c.id)
			state = componentReadyShuttingDown
			switch transition := c.next(state); transition {
			case componentThrowErr:
				goto readyEncounteredFatal
			case componentFinishShutdown:
				goto readyCanceled
			default:
				c.unexpectedStateTransition(state, transition)
			}

		shuttingDown:
			c.t.Logf("[component %v] shutting down\n", c.id)
			state = componentShuttingDown
			switch transition := c.next(state); transition {
			case componentThrowErr:
				goto encounteredFatal
			case componentFinishShutdown:
				goto done
			default:
				c.unexpectedStateTransition(state, transition)
			}

		readyCanceled:
			c.t.Logf("[component %v] ready canceled\n", c.id)
			state = componentReadyCanceled
			return

		done:
			c.t.Logf("[component %v] done\n", c.id)
			state = componentDone
			return

		readyEncounteredFatal:
			c.t.Logf("[component %v] ready encountered fatal\n", c.id)
			state = componentReadyEncounteredFatal
			ctx.Throw(&componentIrrecoverableError{c.id})

		encounteredFatal:
			c.t.Logf("[component %v] encountered fatal\n", c.id)
			state = componentEncounteredFatal
			ctx.Throw(&componentIrrecoverableError{c.id})
		}()

		goto startupFinished
	case componentCheckCtx:
		if isClosed(ctx.Done()) {
			goto startupCanceled
		}
		goto startingUp
	default:
		c.unexpectedStateTransition(state, transition)
	}

startupFailed:
	c.t.Logf("[component %v] startup failed\n", c.id)
	state = componentStartupFailed
	err = &componentStartupError{c.id}
	return

startupCanceled:
	c.t.Logf("[component %v] startup canceled\n", c.id)
	state = componentStartupCanceled
	err = &componentCanceledError{c.id, ctx.Err()}
	return

startupFinished:
	return nil
}

func (c *component) Ready() <-chan struct{} {
	return c.ready
}

func (c *component) Done() <-chan struct{} {
	return c.done
}

type workerState int

const (
	unknownWorkerState = iota
	workerStarted
	workerRunning
	workerShuttingDown
	workerDone
	workerEncounteredFatal
)

func (s workerState) String() string {
	switch s {
	case workerStarted:
		return "worker-started"
	case workerRunning:
		return "worker-running"
	case workerShuttingDown:
		return "worker-shutting-down"
	case workerDone:
		return "worker-done"
	case workerEncounteredFatal:
		return "worker-encountered-fatal"
	default:
		return "unknown"
	}
}

type workerStateList []workerState

func (wsl workerStateList) contains(ws workerState) bool {
	for _, s := range wsl {
		if s == ws {
			return true
		}
	}
	return false
}

type workerStateTransition int

const (
	unknownWorkerStateTransition workerStateTransition = iota
	workerCheckCtx
	workerCheckCtxWithCleanup
	workerThrowErr
	workerFinish
)

func (st workerStateTransition) String() string {
	switch st {
	case workerCheckCtx:
		return "worker-check-ctx"
	case workerCheckCtxWithCleanup:
		return "worker-check-ctx-with-cleanup"
	case workerThrowErr:
		return "worker-throw-err"
	case workerFinish:
		return "worker-finish"
	default:
		return "unknown"
	}
}

type wstConsumer func(workerStateTransition) workerState
type wstProvider func(workerState) workerStateTransition

func componentWorker(t *rapid.T, id int, next wstProvider) ComponentWorker {
	unexpectedStateTransition := func(s workerState, wst workerStateTransition) {
		assert.Fail(t, "unexpected state transition", "[worker %v] received %v for state %v", id, wst, s)
		runtime.Goexit()
	}

	return func(ctx irrecoverable.SignalerContext) {
		var state workerState
		goto started

	started:
		state = workerStarted
		switch transition := next(state); transition {
		case workerCheckCtx:
			if isClosed(ctx.Done()) {
				goto done
			}
			goto running
		default:
			unexpectedStateTransition(state, transition)
		}

	running:
		state = workerRunning
		switch transition := next(state); transition {
		case workerFinish:
			goto done
		case workerThrowErr:
			goto encounteredFatal
		case workerCheckCtxWithCleanup:
			if isClosed(ctx.Done()) {
				goto shuttingDown
			}
			goto running
		case workerCheckCtx:
			if isClosed(ctx.Done()) {
				goto done
			}
			goto running
		default:
			unexpectedStateTransition(state, transition)
		}

	shuttingDown:
		state = workerShuttingDown
		switch transition := next(state); transition {
		case workerFinish:
			goto done
		case workerThrowErr:
			goto encounteredFatal
		default:
			unexpectedStateTransition(state, transition)
		}

	done:
		state = workerDone
		defer next(state)
		return

	encounteredFatal:
		state = workerEncounteredFatal
		defer next(state)
		ctx.Throw(&workerError{id})
	}
}

type startupState int

const (
	unknownStartupState startupState = iota
	startupStarted
	startupRunning
	startupFailed
	startupCanceled
	startupFinished
)

func (s startupState) String() string {
	switch s {
	case startupStarted:
		return "startup-started"
	case startupRunning:
		return "startup-running"
	case startupFailed:
		return "startup-failed"
	case startupCanceled:
		return "startup-canceled"
	case startupFinished:
		return "startup-finished"
	default:
		return "unknown"
	}
}

type startupStateList []startupState

func (ssl startupStateList) contains(ss startupState) bool {
	for _, s := range ssl {
		if s == ss {
			return true
		}
	}
	return false
}

type startupStateTransition int

const (
	unknownStartupStateTransition startupStateTransition = iota
	startupCheckCtx
	startupReturnErr
	startupFinish
)

func (st startupStateTransition) String() string {
	switch st {
	case startupCheckCtx:
		return "startup-check-ctx"
	case startupReturnErr:
		return "startup-return-err"
	case startupFinish:
		return "startup-finish"
	default:
		return "unknown"
	}
}

type sstConsumer func(startupStateTransition) startupState
type sstProvider func(startupState) startupStateTransition

func componentStartup(t *rapid.T, next sstProvider) ComponentStartup {
	unexpectedStateTransition := func(s startupState, sst startupStateTransition) {
		assert.Fail(t, "unexpected state transition", "[startup] received %v for state %v", sst, s)
		runtime.Goexit()
	}

	return func(ctx context.Context) error {
		var state startupState
		var err error
		goto started

	started:
		t.Log("[startup] started\n")
		state = startupStarted
		switch transition := next(state); transition {
		case startupCheckCtx:
			if isClosed(ctx.Done()) {
				goto canceled
			}
			goto running
		default:
			unexpectedStateTransition(state, transition)
		}

	running:
		t.Log("[startup] running\n")
		state = startupRunning
		switch transition := next(state); transition {
		case startupReturnErr:
			err = errStartup
			goto failed
		case startupFinish:
			goto finished
		case startupCheckCtx:
			if isClosed(ctx.Done()) {
				goto canceled
			}
			goto running
		default:
			unexpectedStateTransition(state, transition)
		}

	canceled:
		t.Log("[startup] canceled\n")
		state = startupCanceled
		defer next(state)
		return ctx.Err()

	failed:
		t.Log("[startup] failed\n")
		state = startupFailed
		defer next(state)
		return err

	finished:
		t.Log("[startup] finished\n")
		state = startupFinished
		defer next(state)
		return nil
	}
}

var errStartup = errors.New("startup error")

type componentStartupError struct {
	id int
}

func (e *componentStartupError) Is(target error) bool {
	if t, ok := target.(*componentStartupError); ok {
		return t.id == e.id
	}
	return false
}

func (e *componentStartupError) Error() string {
	return fmt.Sprintf("[component %v] startup error", e.id)
}

type componentCanceledError struct {
	id     int
	ctxErr error
}

func (e *componentCanceledError) Is(target error) bool {
	if t, ok := target.(*componentCanceledError); ok {
		return t.id == e.id && errors.Is(t.ctxErr, e.ctxErr)
	}
	return false
}

func (e *componentCanceledError) Error() string {
	return fmt.Sprintf("[component %v] %v", e.id, e.ctxErr)
}

func (e *componentCanceledError) Unwrap() error {
	return e.ctxErr
}

type componentIrrecoverableError struct {
	id int
}

func (e *componentIrrecoverableError) Is(target error) bool {
	if t, ok := target.(*componentIrrecoverableError); ok {
		return t.id == e.id
	}
	return false
}

func (e *componentIrrecoverableError) Error() string {
	return fmt.Sprintf("[component %v] irrecoverable error", e.id)
}

type workerError struct {
	id int
}

func (e *workerError) Is(target error) bool {
	if t, ok := target.(*workerError); ok {
		return t.id == e.id
	}
	return false
}

func (e *workerError) Error() string {
	return fmt.Sprintf("[worker %v] irrecoverable error", e.id)
}

func makeStartupTransitionFuncs() (sstConsumer, sstProvider) {
	stateChan := make(chan startupState, 1)
	transitionChan := make(chan startupStateTransition)

	consumer := func(sst startupStateTransition) startupState {
		transitionChan <- sst
		return <-stateChan
	}
	provider := func(ss startupState) startupStateTransition {
		if ss != startupStarted {
			stateChan <- ss
		}
		if _, ok := startupStateTransitions[ss]; !ok {
			return unknownStartupStateTransition
		}
		return <-transitionChan
	}

	return consumer, provider
}

func makeComponentTransitionFuncs() (cstConsumer, cstProvider) {
	stateChan := make(chan componentState, 1)
	transitionChan := make(chan componentStateTransition)

	consumer := func(cst componentStateTransition) componentState {
		transitionChan <- cst
		return <-stateChan
	}
	provider := func(cs componentState) componentStateTransition {
		if cs != componentStarted {
			stateChan <- cs
		}
		if _, ok := componentStateTransitions[cs]; !ok {
			return unknownComponentStateTransition
		}
		return <-transitionChan
	}

	return consumer, provider
}

func makeWorkerTransitionFuncs() (wstConsumer, wstProvider) {
	stateChan := make(chan workerState, 1)
	transitionChan := make(chan workerStateTransition)

	consumer := func(wst workerStateTransition) workerState {
		transitionChan <- wst
		return <-stateChan
	}
	provider := func(ws workerState) workerStateTransition {
		if ws != workerStarted {
			stateChan <- ws
		}
		if _, ok := workerStateTransitions[ws]; !ok {
			return unknownWorkerStateTransition
		}
		return <-transitionChan
	}

	return consumer, provider
}

type stateTransition struct {
	cancel bool

	startupStateTransition startupStateTransition

	componentIDs         []int
	componentTransitions []componentStateTransition

	workerIDs         []int
	workerTransitions []workerStateTransition
}

func (st *stateTransition) String() string {
	return fmt.Sprintf(
		"stateTransition{\n"+
			"\tcancel=%v\n"+
			"\tstartupStateTransition=%v\n"+
			"\tcomponentIDs=%v\n"+
			"\tcomponentTransitions=%v\n"+
			"\tworkerIDs=%v\n"+
			"\tworkerTransitions=%v\n"+
			"}",
		st.cancel, st.startupStateTransition, st.componentIDs, st.componentTransitions, st.workerIDs, st.workerTransitions,
	)
}

type componentManagerMachine struct {
	cm *ComponentManager

	cancel                  context.CancelFunc
	checkSignalerError      func() (error, bool)
	checkStartError         func() (error, bool)
	resetChannelReadTimeout func()
	assertClosed            func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{})
	assertNotClosed         func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{})

	cancelGenerator     *rapid.Generator
	drawStateTransition func(t *rapid.T) *stateTransition
	updateStates        func()

	canceled          bool
	componentsStarted bool
	workersStarted    bool

	startupError           error
	componentStartupErrors error
	thrownErrors           error

	startupTransitionConsumer sstConsumer
	startupState              startupState

	components                   []*component
	componentTransitionConsumers []cstConsumer
	componentStates              []componentState

	workerTransitionConsumers []wstConsumer
	workerStates              []workerState
}

func (c *componentManagerMachine) Init(t *rapid.T) {
	hasStartup := rapid.Bool().Draw(t, "has_startup").(bool)
	numWorkers := rapid.IntRange(0, 5).Draw(t, "num_workers").(int)
	numComponents := rapid.IntRange(0, 5).Draw(t, "num_components").(int)
	pCancel := rapid.Float64Range(0, 1).Draw(t, "p_cancel").(float64)

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	signaler := irrecoverable.NewSignaler()
	signalerCtx := irrecoverable.WithSignaler(ctx, signaler)

	cmb := NewComponentManagerBuilder()

	c.cancelGenerator = rapid.Float64Range(0, 1).Map(func(n float64) bool { return pCancel == 1 || n < pCancel })
	c.drawStateTransition = func(t *rapid.T) *stateTransition {
		st := &stateTransition{}

		if !c.canceled {
			st.cancel = c.cancelGenerator.Draw(t, "cancel").(bool)
		}

		if allowedTransitions, ok := startupStateTransitions[c.startupState]; ok {
			st.startupStateTransition = rapid.SampledFrom(allowedTransitions).Draw(t, "startup_state_transition").(startupStateTransition)
		}

		for componentId, state := range c.componentStates {
			if allowedTransitions, ok := componentStateTransitions[state]; ok {
				label := fmt.Sprintf("component_transition_%v", componentId)
				st.componentIDs = append(st.componentIDs, componentId)
				st.componentTransitions = append(st.componentTransitions, rapid.SampledFrom(allowedTransitions).Draw(t, label).(componentStateTransition))
			}
		}

		for workerId, state := range c.workerStates {
			if allowedTransitions, ok := workerStateTransitions[state]; ok {
				label := fmt.Sprintf("worker_transition_%v", workerId)
				st.workerIDs = append(st.workerIDs, workerId)
				st.workerTransitions = append(st.workerTransitions, rapid.SampledFrom(allowedTransitions).Draw(t, label).(workerStateTransition))
			}
		}

		return rapid.Just(st).Draw(t, "state_transition").(*stateTransition)
	}
	c.updateStates = func() {
		if !c.componentsStarted && c.startupState == startupFinished {
			for i := 0; i < len(c.componentStates); i++ {
				c.componentStates[i] = componentStarted
			}
			c.componentsStarted = true
		}

		if !c.workersStarted && c.componentsStarted {
			allComponentsStartupFinished := true
			for _, s := range c.componentStates {
				if !(componentStateList{
					componentStartupFinished,
					componentRunning,
					componentShuttingDown,
					componentDone,
					componentEncounteredFatal,
					componentReadyShuttingDown,
					componentReadyCanceled,
					componentReadyEncounteredFatal,
				}).contains(s) {
					allComponentsStartupFinished = false
					break
				}
			}
			if allComponentsStartupFinished {
				for i := 0; i < len(c.workerStates); i++ {
					c.workerStates[i] = workerStarted
				}
				c.workersStarted = true
			}
		}
	}

	if hasStartup {
		stc, stp := makeStartupTransitionFuncs()

		c.startupTransitionConsumer = stc
		c.startupState = startupStarted

		cmb.OnStart(componentStartup(t, stp))
	}

	c.components = make([]*component, numComponents)
	c.componentTransitionConsumers = make([]cstConsumer, numComponents)
	c.componentStates = make([]componentState, numComponents)

	for i := 0; i < numComponents; i++ {
		ctc, ctp := makeComponentTransitionFuncs()

		c.componentTransitionConsumers[i] = ctc
		c.componentStates[i] = componentIdle
		c.components[i] = newComponent(t, i, ctp)

		cmb.AddComponent(c.components[i])
	}

	c.workerTransitionConsumers = make([]wstConsumer, numWorkers)
	c.workerStates = make([]workerState, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wtc, wtp := makeWorkerTransitionFuncs()

		c.workerTransitionConsumers[i] = wtc
		c.workerStates[i] = unknownWorkerState

		cmb.AddWorker(componentWorker(t, i, wtp))
	}

	c.cm = cmb.Build()
	startErrChan := make(chan error, 1)

	go func() {
		err := c.cm.Start(signalerCtx)
		t.Logf("component manager returned from Start: %v\n", err)
		startErrChan <- err
		close(startErrChan)
	}()

	var channelReadTimeout chan struct{}

	c.resetChannelReadTimeout = func() {
		channelReadTimeout = make(chan struct{})
		ch := channelReadTimeout
		go func() {
			time.Sleep(CHANNEL_CLOSE_LATENCY_ALLOWANCE)
			close(ch)
		}()
	}

	c.assertClosed = func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{}) {
		select {
		case <-ch:
			return
		default:
		}
		select {
		case <-channelReadTimeout:
			assert.Fail(t, "channel is not closed", msgAndArgs...)
		case <-ch:
		}
	}

	c.assertNotClosed = func(t *rapid.T, ch <-chan struct{}, msgAndArgs ...interface{}) {
		select {
		case <-ch:
			assert.Fail(t, "channel is closed", msgAndArgs...)
		default:
		}
		select {
		case <-ch:
			assert.Fail(t, "channel is closed", msgAndArgs...)
		case <-channelReadTimeout:
		}
	}

	var signalerErr error
	var signalerErrOk bool
	c.checkSignalerError = func() (error, bool) {
		if !signalerErrOk {
			select {
			case err := <-signaler.Error():
				signalerErrOk = true
				signalerErr = err
			default:
			}
		}
		return signalerErr, signalerErrOk
	}

	var startErr error
	var startErrOk bool
	c.checkStartError = func() (error, bool) {
		if !startErrOk {
			select {
			case err := <-startErrChan:
				startErrOk = true
				startErr = err
			default:
			}
		}
		return startErr, startErrOk
	}

	if !hasStartup {
		c.startupState = startupFinished
		c.updateStates()
	}
}

func (c *componentManagerMachine) ExecuteStateTransition(t *rapid.T) {
	st := c.drawStateTransition(t)

	t.Logf("drew state transition: %v\n", st)

	var componentStartupErrors *multierror.Error
	var thrownErrors *multierror.Error

	addTransition, executeTransitionsInRandomOrder := startStateTransition()

	if st.cancel {
		addTransition(func() {
			t.Log("executing cancel transition\n")
			c.cancel()
			c.canceled = true
			c.resetChannelReadTimeout()
			c.assertClosed(t, c.cm.ShutdownSignal())
		})
	}

	if st.startupStateTransition != unknownStartupStateTransition {
		addTransition(func() {
			t.Logf("executing startup transition: %v\n", st.startupStateTransition)
			endState := c.startupTransitionConsumer(st.startupStateTransition)
			checkStartupStateTransition(t, c.startupState, endState, st.startupStateTransition, c.canceled)
			c.startupState = endState

			var startupError error
			if endState == startupFailed {
				startupError = errStartup
			} else if endState == startupCanceled {
				startupError = context.Canceled
			}

			if startupError != nil {
				assert.NoError(t, c.startupError)
				c.startupError = startupError
				c.canceled = true
				c.resetChannelReadTimeout()
				c.assertClosed(t, c.cm.ShutdownSignal())
			}
		})
	}

	for i, componentId := range st.componentIDs {
		i := i
		componentId := componentId
		addTransition(func() {
			cst := st.componentTransitions[i]
			t.Logf("executing component %v transition: %v\n", componentId, cst)
			endState := c.componentTransitionConsumers[componentId](cst)
			checkComponentStateTransition(t, componentId, c.componentStates[componentId], endState, cst, c.canceled)
			c.componentStates[componentId] = endState

			var componentStartupErr error
			if endState == componentStartupFailed {
				componentStartupErr = &componentStartupError{componentId}
			} else if endState == componentStartupCanceled {
				componentStartupErr = &componentCanceledError{componentId, context.Canceled}
			}

			if componentStartupErr != nil {
				assert.NotErrorIs(t, c.componentStartupErrors, componentStartupErr)
				assert.NotErrorIs(t, componentStartupErrors, componentStartupErr)
				componentStartupErrors = multierror.Append(componentStartupErrors, componentStartupErr)
				c.canceled = true
				c.resetChannelReadTimeout()
				c.assertClosed(t, c.cm.ShutdownSignal())
			}

			if (componentStateList{componentEncounteredFatal, componentReadyEncounteredFatal}).contains(endState) {
				thrownErr := &componentIrrecoverableError{componentId}
				assert.NotErrorIs(t, c.thrownErrors, thrownErr)
				assert.NotErrorIs(t, thrownErrors, thrownErr)
				thrownErrors = multierror.Append(thrownErrors, thrownErr)
				c.canceled = true
				c.resetChannelReadTimeout()
				c.assertClosed(t, c.cm.ShutdownSignal())
			}
		})
	}

	for i, workerId := range st.workerIDs {
		i := i
		workerId := workerId
		addTransition(func() {
			wst := st.workerTransitions[i]
			t.Logf("executing worker %v transition: %v\n", workerId, wst)
			endState := c.workerTransitionConsumers[workerId](wst)
			checkWorkerStateTransition(t, workerId, c.workerStates[workerId], endState, wst, c.canceled)
			c.workerStates[workerId] = endState

			if endState == workerEncounteredFatal {
				thrownErr := &workerError{workerId}
				assert.NotErrorIs(t, c.thrownErrors, thrownErr)
				assert.NotErrorIs(t, thrownErrors, thrownErr)
				thrownErrors = multierror.Append(thrownErrors, thrownErr)
				c.canceled = true
				c.resetChannelReadTimeout()
				c.assertClosed(t, c.cm.ShutdownSignal())
			}
		})
	}

	executeTransitionsInRandomOrder(t)

	if c.componentStartupErrors == nil {
		c.componentStartupErrors = componentStartupErrors.ErrorOrNil()
	}

	if c.thrownErrors == nil {
		c.thrownErrors = thrownErrors.ErrorOrNil()
	}

	c.updateStates()

	t.Logf("end state: {\n"+
		"\tcanceled=%v\n"+
		"\tcomponentsStarted=%v\n"+
		"\tworkersStarted=%v\n"+
		"\tstartupError=%v\n"+
		"\tcomponentStartupErrors=%v\n"+
		"\tthrownErrors=%v\n"+
		"\tstartupState=%v\n"+
		"\tcomponentStates=%v\n"+
		"\tworkerStates=%v\n"+
		"}\n",
		c.canceled,
		c.componentsStarted,
		c.workersStarted,
		c.startupError,
		c.componentStartupErrors,
		c.thrownErrors,
		c.startupState,
		c.componentStates,
		c.workerStates,
	)

}

func (c *componentManagerMachine) Check(t *rapid.T) {
	time.Sleep(CHANNEL_CLOSE_LATENCY_ALLOWANCE)

	startErr, startErrOk := c.checkStartError()
	thrownErr, thrownErrOk := c.checkSignalerError()

	if c.canceled {
		assert.True(t, isClosed(c.cm.ShutdownSignal()), "context canceled but component manager shutdown signal was not closed")
	}

	sState := c.startupState
	startupDone := (startupStateList{startupCanceled, startupFailed, startupFinished}).contains(sState)
	startupSucceeded := false

	switch sState {
	case startupStarted:
		fallthrough
	case startupRunning:
		assert.False(t, isClosed(c.cm.Done()), "component manager done channel closed before startup function completed")
		assert.False(t, startErrOk, "component manager returned before startup function completed")
		assert.False(t, thrownErrOk, "error was thrown before startup function completed")
	case startupCanceled:
		fallthrough
	case startupFailed:
		assert.True(t, startErrOk, "startup failed but component manager did not return")
		assert.ErrorIs(t, c.startupError, startErr, "error returned from component manager did not match the one returned from startup function")
		assert.True(t, isClosed(c.cm.ShutdownSignal()), "startup failed but context was not canceled")
		assert.True(t, isClosed(c.cm.Done()), "startup failed but component manager done channel not closed")
	case startupFinished:
		startupSucceeded = true
	default:
		assert.FailNow(t, "unexpected startup state", sState)
	}

	allComponentsDone := true
	allComponentsReady := true
	allComponentsStartupSucceeded := true
	anyComponentStartupFailed := false

	for _, cState := range c.componentStates {
		if (componentStateList{
			componentStartupFailed,
			componentStartupCanceled,
		}).contains(cState) {
			anyComponentStartupFailed = true
		} else if !(componentStateList{
			componentDone,
			componentEncounteredFatal,
			componentReadyCanceled,
			componentReadyEncounteredFatal,
		}).contains(cState) {
			allComponentsDone = false
		}
		if !(componentStateList{
			componentRunning,
			componentShuttingDown,
			componentDone,
			componentEncounteredFatal,
		}).contains(cState) {
			allComponentsReady = false
			if !(componentStateList{
				componentStartupFinished,
				componentReadyShuttingDown,
				componentReadyCanceled,
				componentReadyEncounteredFatal,
			}).contains(cState) {
				allComponentsStartupSucceeded = false
			}
		}

		switch cState {
		case componentIdle:
			assert.NotEqual(t, startupFinished, sState)
		case componentStarted:
			fallthrough
		case componentStartingUp:
			assert.False(t, isClosed(c.cm.Done()), "component manager done channel closed before component finished startup")
			assert.False(t, isClosed(c.cm.Ready()), "component manager ready channel closed before component finished startup")
			assert.False(t, startErrOk, "component manager returned before component startup completed")
		case componentStartupCanceled:
			fallthrough
		case componentStartupFailed:
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "component startup failed but context was not canceled")
			assert.False(t, isClosed(c.cm.Ready()), "component manager ready channel closed despite component startup failure")
		case componentStartupFinished:
			assert.False(t, isClosed(c.cm.Done()), "component manager done channel closed before component was done")
			assert.False(t, isClosed(c.cm.Ready()), "component manager ready channel closed before component was ready")
		case componentRunning:
			assert.False(t, isClosed(c.cm.Done()), "component manager done channel closed while component still running")
		case componentShuttingDown:
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "component shutting down but context was not canceled")
			assert.False(t, isClosed(c.cm.Done()), "component manager done channel closed before component finished shutting down")
		case componentDone:
			// component only reaches this state if it was canceled or it encountered a fatal error
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "component done but context was not canceled")
		case componentEncounteredFatal:
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "component encountered fatal but context was not canceled")
		case componentReadyShuttingDown:
			fallthrough
		case componentReadyCanceled:
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "component was canceled before ready but context was not canceled")
			assert.False(t, isClosed(c.cm.Ready()), "component manager ready channel closed but component was canceled before ready")
		case componentReadyEncounteredFatal:
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "component encountered fatal before ready but context was not canceled")
			assert.False(t, isClosed(c.cm.Ready()), "component manager ready channel closed but component encountered fatal before ready")
		default:
			assert.FailNow(t, "unexpected component state", cState)
		}
	}

	if anyComponentStartupFailed && allComponentsDone {
		assert.True(t, startErrOk, "startup failed but component manager did not return")
		assert.ErrorIs(t, c.componentStartupErrors, startErr)
		assert.True(t, isClosed(c.cm.Done()), "startup failed but component manager done channel not closed")
	}

	if startupSucceeded && allComponentsStartupSucceeded {
		assert.True(t, startErrOk, "startup succeeded but component manager did not return")
		assert.NoError(t, startErr)
	}

	if startupSucceeded && allComponentsReady {
		assert.True(t, isClosed(c.cm.Ready()), "all components ready but component manager ready channel not closed")
	}

	allWorkersDone := true

	for _, wState := range c.workerStates {
		if !(workerStateList{workerDone, workerEncounteredFatal}).contains(wState) {
			allWorkersDone = false
		}

		switch wState {
		case unknownWorkerState:
			assert.NotEqual(t, true, startupSucceeded && allComponentsStartupSucceeded)
		case workerStarted:
			fallthrough
		case workerRunning:
			assert.False(t, isClosed(c.cm.Done()), "component manager done channel closed before worker finished running")
		case workerShuttingDown:
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "worker shutting down but context was not canceled")
			assert.False(t, isClosed(c.cm.Done()), "component manager done channel closed before worker finished shutting down")
		case workerDone:
		case workerEncounteredFatal:
			assert.True(t, isClosed(c.cm.ShutdownSignal()), "worker encountered fatal but context was not canceled")
		default:
			assert.FailNow(t, "unexpected worker state", wState)
		}
	}

	if c.thrownErrors != nil {
		assert.True(t, thrownErrOk, "fatal error thrown but was not received by signaler")
		assert.ErrorIs(t, c.thrownErrors, thrownErr)
		assert.True(t, isClosed(c.cm.ShutdownSignal()), "fatal error thrown but context was not canceled")
	} else {
		assert.False(t, thrownErrOk, "received unexpected error from signaler", thrownErr)
	}

	if startupDone && allComponentsDone && allWorkersDone {
		assert.True(t, isClosed(c.cm.Done()), "all components and workers done but component manager done channel not closed")
	}
}

func TestComponentManager(t *testing.T) {
	// skip because this test takes too long
	// t.Skip()
	rapid.Check(t, rapid.Run(&componentManagerMachine{}))
}

func startStateTransition() (func(t func()), func(*rapid.T)) {
	var transitions []func()

	addTransition := func(t func()) {
		transitions = append(transitions, t)
	}
	executeTransitions := func(t *rapid.T) {
		n := rapid.IntRange(0, len(transitions)).Draw(t, "num_transitions_to_execute").(int)
		for i := 0; i < n; i++ {
			j := rapid.IntRange(0, len(transitions)-i-1).Draw(t, "").(int)
			transitions[i], transitions[j+i] = transitions[j+i], transitions[i]
			transitions[i]()
		}
		// TODO: is this simpler?
		// executionOrder := rapid.SliceOfNDistinct(
		// 	rapid.IntRange(0, len(transitions)-1), 0, len(transitions), nil,
		// ).Draw(t, "transition_execution_order").([]int)
		// for _, i := range executionOrder {
		// 	transitions[i]()
		// }
	}

	return addTransition, executeTransitions
}

var componentStateTransitions = map[componentState][]componentStateTransition{
	componentStarted:           {componentCheckCtx},
	componentStartingUp:        {componentCheckCtx, componentFailStartup, componentFinishStartup, componentBecomeReady},
	componentStartupFinished:   {componentCheckCtx, componentCheckCtxWithCleanup, componentThrowErr, componentBecomeReady},
	componentRunning:           {componentCheckCtx, componentCheckCtxWithCleanup, componentThrowErr},
	componentShuttingDown:      {componentThrowErr, componentFinishShutdown},
	componentReadyShuttingDown: {componentThrowErr, componentFinishShutdown},
}

func checkComponentStateTransition(t *rapid.T, id int, start, end componentState, transition componentStateTransition, canceled bool) {
	if !(func() bool {
		switch start {
		case componentStarted:
			switch transition {
			case componentCheckCtx:
				return (!canceled && end == componentStartingUp) || (canceled && end == componentStartupCanceled)
			}
		case componentStartingUp:
			switch transition {
			case componentCheckCtx:
				return (!canceled && end == start) || (canceled && end == componentStartupCanceled)
			case componentFailStartup:
				return end == componentStartupFailed
			case componentFinishStartup:
				return end == componentStartupFinished
			case componentBecomeReady:
				return end == componentRunning
			}
		case componentStartupFinished:
			switch transition {
			case componentCheckCtx:
				return (!canceled && end == start) || (canceled && end == componentReadyCanceled)
			case componentCheckCtxWithCleanup:
				return (!canceled && end == start) || (canceled && end == componentReadyShuttingDown)
			case componentThrowErr:
				return end == componentReadyEncounteredFatal
			case componentBecomeReady:
				return end == componentRunning
			}
		case componentRunning:
			switch transition {
			case componentCheckCtx:
				return (!canceled && end == start) || (canceled && end == componentDone)
			case componentCheckCtxWithCleanup:
				return (!canceled && end == start) || (canceled && end == componentShuttingDown)
			case componentThrowErr:
				return end == componentEncounteredFatal
			}
		case componentShuttingDown:
			switch transition {
			case componentFinishShutdown:
				return end == componentDone
			case componentThrowErr:
				return end == componentEncounteredFatal
			}
		case componentReadyShuttingDown:
			switch transition {
			case componentFinishShutdown:
				return end == componentReadyCanceled
			case componentThrowErr:
				return end == componentReadyEncounteredFatal
			}
		}

		return false
	}()) {
		assert.Fail(t, "invalid component state transition", "[component %v] canceled=%v, transition=%v, start=%v, end=%v", id, canceled, transition, start, end)
	}
}

var workerStateTransitions = map[workerState][]workerStateTransition{
	workerStarted:      {workerCheckCtx},
	workerRunning:      {workerCheckCtx, workerCheckCtxWithCleanup, workerThrowErr, workerFinish},
	workerShuttingDown: {workerThrowErr, workerFinish},
}

func checkWorkerStateTransition(t *rapid.T, id int, start, end workerState, transition workerStateTransition, canceled bool) {
	if !(func() bool {
		switch start {
		case workerStarted:
			switch transition {
			case workerCheckCtx:
				return (!canceled && end == workerRunning) || (canceled && end == workerDone)
			}
		case workerRunning:
			switch transition {
			case workerCheckCtx:
				return (!canceled && end == start) || (canceled && end == workerDone)
			case workerCheckCtxWithCleanup:
				return (!canceled && end == start) || (canceled && end == workerShuttingDown)
			case workerThrowErr:
				return end == workerEncounteredFatal
			case workerFinish:
				return end == workerDone
			}
		case workerShuttingDown:
			switch transition {
			case workerThrowErr:
				return end == workerEncounteredFatal
			case workerFinish:
				return end == workerDone
			}
		}

		return false
	}()) {
		assert.Fail(t, "invalid worker state transition", "[worker %v] canceled=%v, transition=%v, start=%v, end=%v", id, canceled, transition, start, end)
	}
}

var startupStateTransitions = map[startupState][]startupStateTransition{
	startupStarted: {startupCheckCtx},
	startupRunning: {startupCheckCtx, startupReturnErr, startupFinish},
}

func checkStartupStateTransition(t *rapid.T, start, end startupState, transition startupStateTransition, canceled bool) {
	if !(func() bool {
		switch start {
		case startupStarted:
			switch transition {
			case startupCheckCtx:
				return (!canceled && end == startupRunning) || (canceled && end == startupCanceled)
			}
		case startupRunning:
			switch transition {
			case startupCheckCtx:
				return (!canceled && end == start) || (canceled && end == startupCanceled)
			case startupReturnErr:
				return end == startupFailed
			case startupFinish:
				return end == startupFinished
			}
		}

		return false
	}()) {
		assert.Fail(t, "invalid startup state transition", "canceled=%v, transition=%v, start=%v, end=%v", canceled, transition, start, end)
	}
}
