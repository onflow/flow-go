package irrecoverable

import (
	"context"
	"log"
	"runtime"
)

// Signaler sends the error out
type Signaler struct {
	errors chan<- error
}

func NewSignaler(errors chan<- error) *Signaler {
	return &Signaler{errors}
}

// ThrowError is a narrow drop-in replacement for panic, log.Fatal, log.Panic, etc
// anywhere there's something connected to the error channel
func (e *Signaler) ThrowError(err error) {
	e.errors <- err
	runtime.Goexit()
}

// We define a constrained interface to provide a drop-in replacement for context.Context
// including in interfaces that compose it.
type SignalerContext interface {
	context.Context
	ThrowError(err error) // delegates to the signaler
	sealed()              // private, to constrain builder to using WithSignal
}

// private, to force context derivation / WithSignal
type signalerCtxt struct {
	context.Context
	signaler *Signaler
}

func (sc signalerCtxt) sealed() {}

// Drop-in replacement for panic, log.Fatal, log.Panic, etc
// to use when we are able to get an SignalerContext and thread it down in the component
func (sc signalerCtxt) ThrowError(err error) {
	sc.signaler.ThrowError(err)
}

// the One True Way of getting a SignalerContext
func WithSignaler(ctx context.Context, sig *Signaler) SignalerContext {
	return signalerCtxt{ctx, sig}
}

// If we have an SignalerContext, we can directly ctx.ThrowError.
//
// But a lot of library methods expect context.Context, & we want to pass the same w/o boilerplate
// Moreover, we could have built with: context.WithCancel(irrecoverable.WithSignaler(ctx, sig)),
// "downcasting" to context.Context. Yet, we can still type-assert and recover.
//
// ThrowIrrecoverableError can be a drop-in replacement anywhere we have a context.Context likely
// to support Irrecoverables. Note: this is not a method
func ThrowIrrecoverableError(ctx context.Context, err error) {
	signalerAbleContext, ok := ctx.(SignalerContext)
	if ok {
		signalerAbleContext.ThrowError(err)
	}
	// Be spectacular on how this does not -but should- handle irrecoverables:
	log.Fatalf("Irrecoverable error signaler not found for context, please implement! Unhandled irrecoverable error %v", err)
}
