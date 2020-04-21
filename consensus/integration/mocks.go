package integration

import (
	"fmt"
	"sync"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Headers struct {
	sync.Mutex
	headers map[flow.Identifier]*flow.Header
}

func NewHeaders() *Headers {
	return &Headers{
		headers: make(map[flow.Identifier]*flow.Header),
	}
}

func (h *Headers) Store(header *flow.Header) error {
	h.Lock()
	defer h.Unlock()
	h.headers[header.ID()] = header
	return nil
}

func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	header, found := h.headers[blockID]
	if found {
		return header, nil
	}
	return nil, fmt.Errorf("can not find header by id: %v", blockID)
}

func (h *Headers) ByNumber(number uint64) (*flow.Header, error) {
	return nil, nil
}

type Builder struct {
	headers *Headers
}

func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header)) (*flow.Header, error) {
	headers := b.headers.headers
	parent, ok := headers[parentID]
	if !ok {
		return nil, fmt.Errorf("parent block not found (parent: %x)", parentID)
	}
	header := &flow.Header{
		ChainID:     "chain",
		ParentID:    parentID,
		Height:      parent.Height + 1,
		PayloadHash: unittest.IdentifierFixture(),
		Timestamp:   time.Now().UTC(),
	}
	setter(header)
	headers[header.ID()] = header
	return header, nil
}
