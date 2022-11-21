package generic

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type Payload interface {
	*flow.Payload | *cluster.Payload
}

type Block interface {
	*flow.Block | *cluster.Payload
}

//type Block[P Payload] struct {
//	Header  *flow.Header
//	Payload P
//}

//func NoBlock[P Payload]() Block[P] {
//	return Block[P]{}
//}
