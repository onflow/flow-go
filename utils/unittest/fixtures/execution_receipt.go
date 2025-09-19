package fixtures

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionReceipt is the default options factory for [flow.ExecutionReceipt] generation.
var ExecutionReceipt executionReceiptFactory

type executionReceiptFactory struct{}

type ExecutionReceiptOption func(*ExecutionReceiptGenerator, *flow.ExecutionReceipt)

// WithExecutorID is an option that sets the `ExecutorID` of the execution receipt.
func (f executionReceiptFactory) WithExecutorID(executorID flow.Identifier) ExecutionReceiptOption {
	return func(g *ExecutionReceiptGenerator, receipt *flow.ExecutionReceipt) {
		receipt.ExecutorID = executorID
	}
}

// WithExecutionResult is an option that sets the `ExecutionResult` of the execution receipt.
func (f executionReceiptFactory) WithExecutionResult(result flow.ExecutionResult) ExecutionReceiptOption {
	return func(g *ExecutionReceiptGenerator, receipt *flow.ExecutionReceipt) {
		receipt.ExecutionResult = result
	}
}

// WithSpocks is an option that sets the `Spocks` of the execution receipt.
func (f executionReceiptFactory) WithSpocks(spocks ...crypto.Signature) ExecutionReceiptOption {
	return func(g *ExecutionReceiptGenerator, receipt *flow.ExecutionReceipt) {
		receipt.Spocks = spocks
	}
}

// WithExecutorSignature is an option that sets the `ExecutorSignature` of the execution receipt.
func (f executionReceiptFactory) WithExecutorSignature(executorSignature crypto.Signature) ExecutionReceiptOption {
	return func(g *ExecutionReceiptGenerator, receipt *flow.ExecutionReceipt) {
		receipt.ExecutorSignature = executorSignature
	}
}

// ExecutionReceiptGenerator generates execution receipts with consistent randomness.
type ExecutionReceiptGenerator struct {
	executionReceiptFactory

	random           *RandomGenerator
	identifiers      *IdentifierGenerator
	executionResults *ExecutionResultGenerator
	signatures       *SignatureGenerator
}

// NewExecutionReceiptGenerator creates a new ExecutionReceiptGenerator.
func NewExecutionReceiptGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	executionResults *ExecutionResultGenerator,
	signatures *SignatureGenerator,
) *ExecutionReceiptGenerator {
	return &ExecutionReceiptGenerator{
		random:           random,
		identifiers:      identifiers,
		executionResults: executionResults,
		signatures:       signatures,
	}
}

// Fixture generates a [flow.ExecutionReceipt] with random data based on the provided options.
func (g *ExecutionReceiptGenerator) Fixture(opts ...ExecutionReceiptOption) *flow.ExecutionReceipt {
	receipt := &flow.ExecutionReceipt{
		UnsignedExecutionReceipt: flow.UnsignedExecutionReceipt{
			ExecutorID:      g.identifiers.Fixture(),
			ExecutionResult: *g.executionResults.Fixture(),
			Spocks:          g.signatures.List(g.random.IntInRange(1, 5)),
		},
		ExecutorSignature: g.signatures.Fixture(),
	}

	for _, opt := range opts {
		opt(g, receipt)
	}

	return receipt
}

// List generates a list of [flow.ExecutionReceipt].
func (g *ExecutionReceiptGenerator) List(n int, opts ...ExecutionReceiptOption) []*flow.ExecutionReceipt {
	receipts := make([]*flow.ExecutionReceipt, n)
	for i := range n {
		receipts[i] = g.Fixture(opts...)
	}
	return receipts
}

// ExecutionReceiptStub is the default options factory for [flow.ExecutionReceiptStub] generation.
var ExecutionReceiptStub executionReceiptStubFactory

type executionReceiptStubFactory struct{}

type ExecutionReceiptStubOption func(*ExecutionReceiptStubGenerator, *flow.ExecutionReceiptStub)

// WithExecutorID is an option that sets the `ExecutorID` of the execution receipt stub.
func (f executionReceiptStubFactory) WithExecutorID(executorID flow.Identifier) ExecutionReceiptStubOption {
	return func(g *ExecutionReceiptStubGenerator, stub *flow.ExecutionReceiptStub) {
		stub.ExecutorID = executorID
	}
}

// WithResultID is an option that sets the `ResultID` of the execution receipt stub.
func (f executionReceiptStubFactory) WithResultID(resultID flow.Identifier) ExecutionReceiptStubOption {
	return func(g *ExecutionReceiptStubGenerator, stub *flow.ExecutionReceiptStub) {
		stub.ResultID = resultID
	}
}

// WithSpocks is an option that sets the `Spocks` of the execution receipt stub.
func (f executionReceiptStubFactory) WithSpocks(spocks ...crypto.Signature) ExecutionReceiptStubOption {
	return func(g *ExecutionReceiptStubGenerator, stub *flow.ExecutionReceiptStub) {
		stub.Spocks = spocks
	}
}

// WithExecutorSignature is an option that sets the `ExecutorSignature` of the execution receipt stub.
func (f executionReceiptStubFactory) WithExecutorSignature(executorSignature crypto.Signature) ExecutionReceiptStubOption {
	return func(g *ExecutionReceiptStubGenerator, stub *flow.ExecutionReceiptStub) {
		stub.ExecutorSignature = executorSignature
	}
}

// ExecutionReceiptStubGenerator generates execution receipt stubs with consistent randomness.
type ExecutionReceiptStubGenerator struct {
	executionReceiptStubFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
	signatures  *SignatureGenerator
}

// NewExecutionReceiptStubGenerator creates a new ExecutionReceiptStubGenerator.
func NewExecutionReceiptStubGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	signatures *SignatureGenerator,
) *ExecutionReceiptStubGenerator {
	return &ExecutionReceiptStubGenerator{
		random:      random,
		identifiers: identifiers,
		signatures:  signatures,
	}
}

// Fixture generates a [flow.ExecutionReceiptStub] with random data based on the provided options.
func (g *ExecutionReceiptStubGenerator) Fixture(opts ...ExecutionReceiptStubOption) *flow.ExecutionReceiptStub {
	stub := &flow.ExecutionReceiptStub{
		UnsignedExecutionReceiptStub: flow.UnsignedExecutionReceiptStub{
			ExecutorID: g.identifiers.Fixture(),
			ResultID:   g.identifiers.Fixture(),
			Spocks:     g.signatures.List(g.random.IntInRange(1, 5)),
		},
		ExecutorSignature: g.signatures.Fixture(),
	}

	for _, opt := range opts {
		opt(g, stub)
	}

	return stub
}

// List generates a list of [flow.ExecutionReceiptStub].
func (g *ExecutionReceiptStubGenerator) List(n int, opts ...ExecutionReceiptStubOption) []*flow.ExecutionReceiptStub {
	stubs := make([]*flow.ExecutionReceiptStub, n)
	for i := range n {
		stubs[i] = g.Fixture(opts...)
	}
	return stubs
}
