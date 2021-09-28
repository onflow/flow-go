package validators

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type view struct {
	Parent *view
	Ledger *payloadLedger
}

func newView(payloads []ledger.Payload) *view {
	return &view{
		Ledger: newLed(payloads),
	}
}

func (v *view) NewChild() state.View {
	payload := make([]ledger.Payload, 0)
	ch := newView(payload)
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

func (v *view) Set(owner, controller, key string, value flow.RegisterValue) error {
	return v.Ledger.Set(owner, controller, key, value)
}

func (v *view) Get(owner, controller, key string) (flow.RegisterValue, error) {
	value, err := v.Ledger.Get(owner, controller, key)
	if err != nil {
		return nil, err
	}
	if len(value) > 0 {
		return value, nil
	}

	if v.Parent != nil {
		return v.Parent.Get(owner, controller, key)
	}

	return nil, nil
}

func (v *view) Touch(owner, controller, key string) error {
	return v.Ledger.Touch(owner, controller, key)
}

func (v *view) Delete(owner, controller, key string) error {
	return v.Ledger.Delete(owner, controller, key)
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

type payloadLedger struct {
	payloads map[string]ledger.Payload
}

func (l *payloadLedger) Set(owner, controller, key string, value flow.RegisterValue) error {
	keyparts := []ledger.KeyPart{ledger.NewKeyPart(0, []byte(owner)),
		ledger.NewKeyPart(1, []byte(controller)),
		ledger.NewKeyPart(2, []byte(key))}
	fk := fullKey(owner, controller, key)
	l.payloads[fk] = ledger.Payload{Key: ledger.NewKey(keyparts), Value: ledger.Value(value)}
	return nil
}

func (l *payloadLedger) Get(owner, controller, key string) (flow.RegisterValue, error) {
	fk := fullKey(owner, controller, key)
	return flow.RegisterValue(l.payloads[fk].Value), nil
}

func (l *payloadLedger) Delete(owner, controller, key string) error {
	fk := fullKey(owner, controller, key)
	delete(l.payloads, fk)
	return nil
}

func (l *payloadLedger) Touch(owner, controller, key string) error {
	return nil
}

func (l *payloadLedger) Payloads() []ledger.Payload {
	ret := make([]ledger.Payload, 0, len(l.payloads))
	for _, v := range l.payloads {
		ret = append(ret, v)
	}
	return ret
}

func newLed(payloads []ledger.Payload) *payloadLedger {
	mapping := make(map[string]ledger.Payload)
	for _, p := range payloads {
		fk := fullKey(string(p.Key.KeyParts[0].Value),
			string(p.Key.KeyParts[1].Value),
			string(p.Key.KeyParts[2].Value))
		mapping[fk] = p
	}

	return &payloadLedger{
		payloads: mapping,
	}
}

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
