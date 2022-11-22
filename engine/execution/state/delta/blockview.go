package delta

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// BlockView is simplified version of view (no spock secrete computation or register touch tracking)
// TODO maybe adding a read cache
type BlockView struct {
	Delta    Delta
	ReadFunc GetRegisterFunc
}

func (v *BlockView) Get(owner, key string) (flow.RegisterValue, error) {
	var err error
	value, exists := v.Delta.Get(owner, key)
	if !exists {
		value, err = v.ReadFunc(owner, key)
		if err != nil {
			return nil, fmt.Errorf("get register failed: %w", err)
		}
	}
	return value, nil
}

func (v *BlockView) Set(owner, key string, value flow.RegisterValue) error {
	v.Delta.Set(owner, key, value)
	return nil
}

func (v *BlockView) Touch(owner, key string) error {
	return nil
}

func (v *BlockView) Delete(owner, key string) error {
	return v.Set(owner, key, nil)
}

func NewBlockView(readFunc GetRegisterFunc) *BlockView {
	return &BlockView{
		Delta:    NewDelta(),
		ReadFunc: readFunc,
	}
}
