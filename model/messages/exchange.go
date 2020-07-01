package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	ResourceBlock Resource = iota
	ResourceHeader
	ResourcePayload
	ResourceSeal
	ResourceReceipt
	ResourceApproval
)

type Resource uint8

func (r Resource) String() string {
	switch r {
	case ResourceBlock:
		return "block"
	case ResourceHeader:
		return "header"
	case ResourcePayload:
		return "payload"
	case ResourceSeal:
		return "seal"
	case ResourceReceipt:
		return "receipt"
	case ResourceApproval:
		return "approval"
	default:
		return "invalid"
	}
}

type ResourceRequest struct {
	Resource Resource
	EntityID flow.Identifier
	Nonce    uint64
}

type ResourceResponse struct {
	Resource Resource
	Entity   flow.Entity
	Nonce    uint64
}
