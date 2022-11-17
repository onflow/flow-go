package utils

import (
	"github.com/google/btree"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type LedgerRegister struct {
	Key     flow.RegisterID
	Value   flow.RegisterValue
	Touched bool
	Updated bool
}

type BTreeLedger struct {
	Tree *btree.BTreeG[LedgerRegister]
}

func (b *BTreeLedger) Set(owner, key string, value flow.RegisterValue) error {
	b.Tree.ReplaceOrInsert(LedgerRegister{Key: flow.RegisterID{Owner: owner, Key: key}, Value: value, Touched: true, Updated: true})
	return nil
}

func (b *BTreeLedger) Get(owner, key string) (flow.RegisterValue, error) {
	r := LedgerRegister{Key: flow.RegisterID{Owner: owner, Key: key}, Touched: true}
	p, ok := b.Tree.Get(r)
	if !ok {
		b.Tree.ReplaceOrInsert(r)
		return nil, nil
	}

	if p.Touched {
		return p.Value, nil
	}

	p.Touched = true
	b.Tree.ReplaceOrInsert(p)
	return p.Value, nil
}

func (b *BTreeLedger) Touch(owner, key string) error {
	_, err := b.Get(owner, key)
	return err
}

func (b *BTreeLedger) Delete(owner, key string) error {
	b.Tree.Delete(LedgerRegister{Key: flow.RegisterID{Owner: owner, Key: key}})
	return nil
}

func NewBTreeView() *BTreeView {
	return &BTreeView{
		Ledger: BTreeLedger{
			Tree: btree.NewG(32, func(a, b LedgerRegister) bool {
				return (a.Key.Owner < b.Key.Owner) || (a.Key.Owner == b.Key.Owner && a.Key.Key < b.Key.Key)
			}),
		},
	}
}

type BTreeView struct {
	Parent BTreeLedger
	Ledger BTreeLedger
}

func (v *BTreeView) NewChild() state.View {
	return &BTreeView{
		Ledger: BTreeLedger{
			Tree: v.Ledger.Tree.Clone(),
		},
		Parent: BTreeLedger{
			Tree: v.Ledger.Tree,
		},
	}
}

func (v *BTreeView) MergeView(o state.View) error {
	v.Ledger.Tree = o.(*BTreeView).Ledger.Tree
	return nil
}

func (v *BTreeView) DropDelta() {
	v.Ledger.Tree = v.Parent.Tree.Clone()
}

// AllRegisters returns all the registers that has been touched
func (v *BTreeView) AllRegisters() []flow.RegisterID {
	res := make([]flow.RegisterID, 0, v.Ledger.Tree.Len())
	v.Ledger.Tree.Ascend(func(i LedgerRegister) bool {
		if i.Touched {
			res = append(res, i.Key)
		}
		return true
	})
	return res
}

func (v *BTreeView) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	len := v.Ledger.Tree.Len()
	ids := make([]flow.RegisterID, 0, len)
	values := make([]flow.RegisterValue, 0, len)

	v.Ledger.Tree.Ascend(func(i LedgerRegister) bool {
		if i.Updated {
			ids = append(ids, i.Key)
			values = append(values, i.Value)
		}
		return true
	})
	return ids, values
}

func (v *BTreeView) Get(owner, key string) (flow.RegisterValue, error) {
	value, err := v.Ledger.Get(owner, key)
	if err != nil {
		return nil, err
	}
	if len(value) > 0 {
		return value, nil
	}
	return nil, nil
}

func (v *BTreeView) Set(owner, key string, value flow.RegisterValue) error {
	return v.Ledger.Set(owner, key, value)
}

func (v *BTreeView) Touch(owner, key string) error {
	return v.Ledger.Touch(owner, key)
}

func (v *BTreeView) Delete(owner, key string) error {
	return v.Ledger.Delete(owner, key)
}
