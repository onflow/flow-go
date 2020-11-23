package migrations

import (
	"strings"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func AddMissingKeysMigration(payloads []ledger.Payload) ([]ledger.Payload, error) {
	l := newLed(payloads)
	a := state.NewAccounts(l)
	// fungible token
	ok, err := a.Exists(flow.HexToAddress("f233dcee88fe0abe"))
	if err != nil {
		return nil, err
	}
	if ok {
		// a.AppendPublicKey()
	}
	return l.Payloads(), nil
}

type led struct {
	payloads map[string]ledger.Payload
}

func (l *led) Set(owner, controller, key string, value flow.RegisterValue) {
	keyparts := []ledger.KeyPart{ledger.NewKeyPart(0, []byte(owner)),
		ledger.NewKeyPart(1, []byte(controller)),
		ledger.NewKeyPart(2, []byte(key))}
	fk := fullKey(owner, controller, key)
	l.payloads[fk] = ledger.Payload{Key: ledger.NewKey(keyparts), Value: ledger.Value(value)}
}

func (l *led) Get(owner, controller, key string) (flow.RegisterValue, error) {
	fk := fullKey(owner, controller, key)
	return flow.RegisterValue(l.payloads[fk].Value), nil
}

func (l *led) Delete(owner, controller, key string) {
	fk := fullKey(owner, controller, key)
	delete(l.payloads, fk)
}

func (l *led) Touch(owner, controller, key string) {}

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
		fk := fullKey(string(p.Key.KeyParts[0].Value),
			string(p.Key.KeyParts[1].Value),
			string(p.Key.KeyParts[2].Value))
		mapping[fk] = p
	}

	return &led{
		payloads: mapping,
	}
}

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
