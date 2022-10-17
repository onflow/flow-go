package internal

import "github.com/onflow/flow-go/model/flow"

// RequestEntity is a wrapper around EntityRequest that implements Entity interface for it, and
// also internally caches its identifier.
type RequestEntity struct {
	// requested entity ids
	EntityIDs []flow.Identifier

	// identifier of the requester.
	OriginId flow.Identifier

	// caching identifier to avoid cpu overhead per query.
	id flow.Identifier
}

func NewRequestEntity(originId flow.Identifier, entityIds []flow.Identifier) RequestEntity {
	return RequestEntity{
		OriginId:  originId,
		EntityIDs: entityIds,
		id:        identifierOfRequest(originId, entityIds),
	}
}

func (r RequestEntity) ID() flow.Identifier {
	return r.id
}

func (r RequestEntity) Checksum() flow.Identifier {
	return r.id
}

func identifierOfRequest(originId flow.Identifier, entityIds []flow.Identifier) flow.Identifier {
	return flow.MakeID(append([]flow.Identifier{originId}, entityIds...))
}
