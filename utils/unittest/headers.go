package unittest

import (
	"fmt"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
)

// HeadersFromMap creates a storage header mock that backed by a given map
func HeadersFromMap(headerDB map[flow.Identifier]*flow.Header) *storage.Headers {
	headers := &storage.Headers{}
	headers.On("Store", mock.Anything).Return(
		func(header *flow.Header) error {
			headerDB[header.ID()] = header
			return nil
		},
	)
	headers.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			return headerDB[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := headerDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	return headers
}

var Header headerFactory

type headerFactory struct{}

// HeaderFixture initializes and returns a new *flow.Header instance.
func HeaderFixture(opts ...func(*flow.Header)) *flow.Header {
	header := BlockHeaderFixture()
	for _, opt := range opts {
		opt(header)
	}
	return header
}

// WithParent is an option for the [unittest.Header] factory. It updates the fields `ParentID`, `ParentView`,
// to match the parent's values. Furthermore, it sets the `View` and `Height` of the header to be one greater
// than the parent's values. This emulates the happy path and hence is a sensible default.
func (f *headerFactory) WithParent(parentID flow.Identifier, parentView uint64, parentHeight uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ParentID = parentID
		header.ParentView = parentView
		header.View = parentView + 1
		header.Height = parentHeight + 1
	}
}

func (f *headerFactory) WithView(view uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.View = view
	}
}

func (f *headerFactory) WithParentView(view uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ParentView = view
	}
}

func (f *headerFactory) WithHeight(height uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.Height = height
	}
}

func (f *headerFactory) WithChainID(chainID flow.ChainID) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ChainID = chainID
	}
}

func (f *headerFactory) WithProposerID(proposerID flow.Identifier) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ProposerID = proposerID
	}
}

func (f *headerFactory) WithPayloadHash(payloadHash flow.Identifier) func(*flow.Header) {
	return func(header *flow.Header) {
		header.PayloadHash = payloadHash
	}
}

func (f *headerFactory) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*flow.Header) {
	return func(header *flow.Header) {
		header.LastViewTC = lastViewTC
	}
}

func (f *headerFactory) Genesis(chainID flow.ChainID) *flow.Header {
	// create the headerBody for the genesis block
	headerBody, err := flow.NewRootHeaderBody(
		flow.UntrustedHeaderBody{
			ChainID:   chainID,
			ParentID:  flow.ZeroID,
			Height:    0,
			Timestamp: uint64(flow.GenesisTime.UnixMilli()),
			View:      0,
		},
	)
	if err != nil {
		panic(fmt.Errorf("failed to create root header body: %w", err))
	}

	// create the header
	header, err := flow.NewRootHeader(
		flow.UntrustedHeader{
			HeaderBody:  *headerBody,
			PayloadHash: IdentifierFixture(),
		},
	)
	if err != nil {
		panic(fmt.Errorf("failed to create root header: %w", err))
	}

	return header
}
