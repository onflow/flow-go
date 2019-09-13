package trampoline

import "github.com/dapperlabs/flow-go/pkg/language/runtime/errors"

// Based on "Stackless Scala With Free" by Rúnar Óli Bjarnason:
// http://blog.higher-order.com/assets/trampolines.pdf
///
/// Trampolines allow computations to be executed in constant stack space,
/// by trading it for heap space. They can be used for computations which
/// would otherwise use a large amount of stack space, potentially crashing
/// when the limited amount is exhausted (stack overflow).
///
/// A Trampoline represents a computation which consists of steps.
/// Each step is either more work which should be executed (`More`),
/// in the form of a function which returns the next step,
/// or a final value (`Done`), which indicates the end of the computation.
///
/// In trampolined programs, instead of each computation invoking
/// the next computation, (i.e., calling functions, possibly recursing directly),
/// they yield the next computation.
///
/// Trampolines can be executed through a control loop using the `Run` method,
/// and can be chained together using the `FlatMap` method.
///

type Trampoline interface {
	Resume() interface{}
	FlatMap(f func(interface{}) Trampoline) Trampoline
	Map(f func(interface{}) interface{}) Trampoline
	Then(f func(interface{})) Trampoline
}

func Run(t Trampoline) interface{} {
	for {
		result := t.Resume()

		if continuation, ok := result.(func() Trampoline); ok {
			t = continuation()
			continue
		}

		return result
	}
}

func MapTrampoline(t Trampoline, f func(interface{}) interface{}) Trampoline {
	return t.FlatMap(func(value interface{}) Trampoline {
		return Done{Result: f(value)}
	})
}

func ThenTrampoline(t Trampoline, f func(interface{})) Trampoline {
	return t.Map(func(value interface{}) interface{} {
		f(value)
		return value
	})
}

// Done

type Done struct {
	Result interface{}
}

func (d Done) Resume() interface{} {
	return d.Result
}

func (d Done) FlatMap(f func(interface{}) Trampoline) Trampoline {
	return FlatMap{Subroutine: d, Continuation: f}
}

func (d Done) Map(f func(interface{}) interface{}) Trampoline {
	return MapTrampoline(d, f)
}

func (d Done) Then(f func(interface{})) Trampoline {
	return ThenTrampoline(d, f)
}

type Continuation interface {
	Continue() Trampoline
}

// More

type More func() Trampoline

func (m More) Resume() interface{} {
	return (func() Trampoline)(m)
}

func (m More) FlatMap(f func(interface{}) Trampoline) Trampoline {
	return FlatMap{Subroutine: m, Continuation: f}
}

func (m More) Map(f func(interface{}) interface{}) Trampoline {
	return MapTrampoline(m, f)
}

func (m More) Then(f func(interface{})) Trampoline {
	return ThenTrampoline(m, f)
}

func (m More) Continue() Trampoline {
	return m()
}

// FlatMap

type FlatMap struct {
	Subroutine   Trampoline
	Continuation func(interface{}) Trampoline
}

func (m FlatMap) FlatMap(f func(interface{}) Trampoline) Trampoline {
	continuation := m.Continuation
	return FlatMap{
		Subroutine: m.Subroutine,
		Continuation: func(value interface{}) Trampoline {
			return continuation(value).FlatMap(f)
		},
	}
}

func (m FlatMap) Resume() interface{} {
	continuation := m.Continuation

	switch sub := m.Subroutine.(type) {
	case Done:
		return func() Trampoline {
			return continuation(sub.Result)
		}
	case Continuation:
		return func() Trampoline {
			return sub.Continue().FlatMap(continuation)
		}
	case FlatMap:
		panic("FlatMap is not a valid subroutine. Use the FlatMap function to construct proper FlatMap structures.")
	}

	panic(&errors.UnreachableError{})
}

func (m FlatMap) Map(f func(interface{}) interface{}) Trampoline {
	return MapTrampoline(m, f)
}

func (m FlatMap) Then(f func(interface{})) Trampoline {
	return ThenTrampoline(m, f)
}
