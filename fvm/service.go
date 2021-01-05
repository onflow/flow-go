package fvm

import (
	"time"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// Blocks is a service which lookup and returns block headers
type Blocks interface {
	// ByHeight returns the block at the given height in the chain ending in `header` (or finalized
	// if `header` is nil). This enables querying un-finalized blocks by height with respect to the
	// chain defined by the block we are executing.
	ByHeightFrom(height uint64, header *flow.Header) (*flow.Header, error)
}

// SignatureVerifier is a signature verification service
type SignatureVerifier interface {
	Verify(
		signature []byte,
		tag []byte,
		message []byte,
		publicKey crypto.PublicKey,
		hashAlgo hash.HashingAlgorithm,
	) (bool, error)

	VerifySignatureFromRuntime(
		signature []byte,
		rawTag string,
		message []byte,
		rawPublicKey []byte,
		rawSigAlgo string,
		rawHashAlgo string,
	) (bool, error)
}

// UUIDGenerator generates UUIDs
type UUIDGenerator interface {
	GenerateUUID() (uint64, error)
}

// A MetricsCollector accumulates performance metrics reported by the Cadence runtime.
//
// A single collector instance will sum all reported values. For example, the "parsed" field will be
// incremented each time a program is parsed.
type MetricsCollector interface {
	Parsed() time.Duration
	Checked() time.Duration
	Interpreted() time.Duration
	ProgramParsed(location ast.Location, duration time.Duration)
	ProgramChecked(location ast.Location, duration time.Duration)
	ProgramInterpreted(location ast.Location, duration time.Duration)
	ValueEncoded(duration time.Duration)
	ValueDecoded(duration time.Duration)
}
