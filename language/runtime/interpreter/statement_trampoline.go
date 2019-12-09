package interpreter

import (
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
)

type StatementTrampoline struct {
	F    func() trampoline.Trampoline
	Line int
}

func (m StatementTrampoline) Resume() interface{} {
	return m.F
}

func (m StatementTrampoline) FlatMap(f func(interface{}) trampoline.Trampoline) trampoline.Trampoline {
	return trampoline.FlatMap{Subroutine: m, Continuation: f}
}

func (m StatementTrampoline) Map(f func(interface{}) interface{}) trampoline.Trampoline {
	return trampoline.MapTrampoline(m, f)
}

func (m StatementTrampoline) Then(f func(interface{})) trampoline.Trampoline {
	return trampoline.ThenTrampoline(m, f)
}

func (m StatementTrampoline) Continue() trampoline.Trampoline {
	return m.F()
}
