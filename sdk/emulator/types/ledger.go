package types

import "github.com/dapperlabs/flow-go/model/flow"

type LedgerDelta struct {
	Updated flow.Ledger
	Deleted map[string]struct{}
}

func (d *LedgerDelta) Keys() []string {
	keys := make([]string, 0, len(d.Updated)+len(d.Deleted))

	for key, _ := range d.Updated {
		keys = append(keys, key)
	}

	for key, _ := range d.Deleted {
		keys = append(keys, key)
	}

	return keys
}

func NewLedgerDelta() *LedgerDelta {
	return &LedgerDelta{
		Updated: make(flow.Ledger),
		Deleted: make(map[string]struct{}),
	}
}

type GetRegisterFunc func(key string) ([]byte, error)

// LedgerView provides a read-only view into an existing ledger set.
//
// Values are written to a temporary register cache that can later be
// committed to the world state.
type LedgerView struct {
	delta    *LedgerDelta
	readFunc GetRegisterFunc
}

// LedgerView
func NewLedgerView(readFunc GetRegisterFunc) *LedgerView {
	return &LedgerView{
		delta:    NewLedgerDelta(),
		readFunc: readFunc,
	}
}

func (r *LedgerView) Delta() *LedgerDelta {
	return r.delta
}

func (r *LedgerView) Get(key string) ([]byte, error) {
	value := r.delta.Updated[key]
	if value != nil {
		return value, nil
	}

	value, err := r.readFunc(key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (r *LedgerView) Set(key string, value []byte) {
	r.delta.Updated[key] = value
}

func (r *LedgerView) Delete(key string) {
	r.delta.Updated[key] = nil
	r.delta.Deleted[key] = struct{}{}
}
