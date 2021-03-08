package common

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type NodeIDsArgs flow.IdentifierList

func (f *NodeIDsArgs) String() string {
	return fmt.Sprint([]string(*f))
}

func (f *NodeIDsArgs) Set(value string) error {
	id := flow.HexToAddress(value)
	f.
	f = append(f, id)
	return nil
}


