package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// Payload is the default options factory for [flow.Payload] generation.
var Payload payloadFactory

type payloadFactory struct{}

type PayloadOption func(*PayloadGenerator, *flow.Payload)

// WithSeals is an option that sets the `Seals` of the payload.
func (f payloadFactory) WithSeals(seals ...*flow.Seal) PayloadOption {
	return func(g *PayloadGenerator, payload *flow.Payload) {
		payload.Seals = seals
	}
}

// WithGuarantees is an option that sets the `Guarantees` of the payload.
func (f payloadFactory) WithGuarantees(guarantees ...*flow.CollectionGuarantee) PayloadOption {
	return func(g *PayloadGenerator, payload *flow.Payload) {
		payload.Guarantees = guarantees
	}
}

// WithReceipts is an option that updates the `Receipts` and `Results` fields of the payload by
// appending all receipts and their `ExecutionResults` to the payload.
//
// To add only receipts, use `WithReceiptStubs` instead.
// e.g.
//
//	Payload.WithReceiptsStubs(
//	    flow.ExecutionReceiptList(receipts).Stubs()...,
//	)
func (f payloadFactory) WithReceipts(receipts ...*flow.ExecutionReceipt) PayloadOption {
	return func(g *PayloadGenerator, payload *flow.Payload) {
		for _, receipt := range receipts {
			payload.Receipts = append(payload.Receipts, receipt.Stub())
			payload.Results = append(payload.Results, &receipt.ExecutionResult)
		}
	}
}

// WithReceiptStubs is an option that sets the `Receipts` of the payload.
func (f payloadFactory) WithReceiptStubs(receipts ...*flow.ExecutionReceiptStub) PayloadOption {
	return func(g *PayloadGenerator, payload *flow.Payload) {
		payload.Receipts = receipts
	}
}

// WithResults is an option that sets the `Results` of the payload.
func (f payloadFactory) WithResults(results ...*flow.ExecutionResult) PayloadOption {
	return func(g *PayloadGenerator, payload *flow.Payload) {
		payload.Results = results
	}
}

// Empty is an option that sets the `Guarantees`, `Receipts`, `Results`, and `Seals` of the payload to nil.
// Equivalent to
//
//	 g.Payloads().Fixture(
//		fixtures.Payload.WithGuarantees(),
//		fixtures.Payload.WithReceiptStubs(),
//		fixtures.Payload.WithResults(),
//		fixtures.Payload.WithSeals(),
//	 )
func (f payloadFactory) Empty() PayloadOption {
	return func(g *PayloadGenerator, payload *flow.Payload) {
		payload.Guarantees = nil
		payload.Receipts = nil
		payload.Results = nil
		payload.Seals = nil
	}
}

// WithProtocolStateID is an option that sets the `ProtocolStateID` of the payload.
func (f payloadFactory) WithProtocolStateID(protocolStateID flow.Identifier) PayloadOption {
	return func(g *PayloadGenerator, payload *flow.Payload) {
		payload.ProtocolStateID = protocolStateID
	}
}

// PayloadGenerator generates block payloads with consistent randomness.
type PayloadGenerator struct {
	payloadFactory

	random                *RandomGenerator
	identifies            *IdentifierGenerator
	guarantees            *CollectionGuaranteeGenerator
	seals                 *SealGenerator
	executionReceiptStubs *ExecutionReceiptStubGenerator
	executionResults      *ExecutionResultGenerator
}

func NewPayloadGenerator(
	random *RandomGenerator,
	identifies *IdentifierGenerator,
	guarantees *CollectionGuaranteeGenerator,
	seals *SealGenerator,
	executionReceiptStubs *ExecutionReceiptStubGenerator,
	executionResults *ExecutionResultGenerator,
) *PayloadGenerator {
	return &PayloadGenerator{
		random:                random,
		identifies:            identifies,
		guarantees:            guarantees,
		seals:                 seals,
		executionReceiptStubs: executionReceiptStubs,
		executionResults:      executionResults,
	}
}

// Fixture generates a [flow.Payload] with random data based on the provided options.
func (g *PayloadGenerator) Fixture(opts ...PayloadOption) *flow.Payload {
	payload := &flow.Payload{
		Guarantees:      g.guarantees.List(g.random.IntInRange(0, 4)),
		Seals:           g.seals.List(g.random.IntInRange(0, 3)),
		Receipts:        g.executionReceiptStubs.List(g.random.IntInRange(0, 10)),
		Results:         g.executionResults.List(g.random.IntInRange(0, 10)),
		ProtocolStateID: g.identifies.Fixture(),
	}

	for _, opt := range opts {
		opt(g, payload)
	}

	return payload
}

// List generates a list of [flow.Payload].
func (g *PayloadGenerator) List(n int, opts ...PayloadOption) []*flow.Payload {
	payloads := make([]*flow.Payload, n)
	for i := range n {
		payloads[i] = g.Fixture(opts...)
	}
	return payloads
}
