package migrations

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type view struct {
	Parent *view
	Ledger *led
}

func NewView(payloads []ledger.Payload) *view {
	return &view{
		Ledger: newLed(payloads),
	}
}

func (v *view) NewChild() state.View {
	payload := make([]ledger.Payload, 0)
	ch := NewView(payload)
	ch.Parent = v
	return ch
}

func (v *view) DropDelta() {
	v.Ledger.payloads = make(map[string]ledger.Payload)
}

func (v *view) MergeView(o state.View) error {
	var other *view
	var ok bool
	if other, ok = o.(*view); !ok {
		return fmt.Errorf("view type mismatch (given: %T, expected:Delta.View)", o)
	}

	for key, value := range other.Ledger.payloads {
		v.Ledger.payloads[key] = value
	}
	return nil
}

func (v *view) Set(owner, key string, value flow.RegisterValue) error {
	return v.Ledger.Set(owner, key, value)
}

func (v *view) Get(owner, key string) (flow.RegisterValue, error) {
	value, err := v.Ledger.Get(owner, key)
	if err != nil {
		return nil, err
	}
	if len(value) > 0 {
		return value, nil
	}

	if v.Parent != nil {
		return v.Parent.Get(owner, key)
	}

	return nil, nil
}

func (v *view) Touch(owner, key string) error {
	return v.Ledger.Touch(owner, key)
}

func (v *view) Delete(owner, key string) error {
	return v.Ledger.Delete(owner, key)
}

func (v *view) Payloads() []ledger.Payload {
	return v.Ledger.Payloads()
}

func (v *view) AllRegisters() []flow.RegisterID {
	panic("AllRegisters is not implemented")
}

func (v *view) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	panic("RegisterUpdates is not implemented")
}

type led struct {
	payloads map[string]ledger.Payload
}

func (l *led) Set(owner, key string, value flow.RegisterValue) error {
	keyparts := []ledger.KeyPart{ledger.NewKeyPart(0, []byte(owner)),
		ledger.NewKeyPart(2, []byte(key))}
	fk := fullKey(owner, key)
	l.payloads[fk] = *ledger.NewPayload(ledger.NewKey(keyparts), ledger.Value(value))
	return nil
}

func (l *led) Get(owner, key string) (flow.RegisterValue, error) {
	fk := fullKey(owner, key)
	p := l.payloads[fk]
	return flow.RegisterValue(p.Value()), nil
}

func (l *led) Delete(owner, key string) error {
	fk := fullKey(owner, key)
	delete(l.payloads, fk)
	return nil
}

func (l *led) Touch(owner, key string) error {
	return nil
}

func (l *led) Payloads() []ledger.Payload {
	ret := make([]ledger.Payload, 0, len(l.payloads))
	for _, v := range l.payloads {
		ret = append(ret, v)
	}
	return ret
}

func newLed(payloads []ledger.Payload) *led {
	mapping := make(map[string]ledger.Payload)
	for _, p := range payloads {
		k, err := p.Key()
		if err != nil {
			panic(err)
		}
		fk := fullKey(string(k.KeyParts[0].Value),
			string(k.KeyParts[1].Value))
		mapping[fk] = p
	}

	return &led{
		payloads: mapping,
	}
}

func fullKey(owner, key string) string {
	return strings.Join([]string{owner, key}, "\x1F")
}
