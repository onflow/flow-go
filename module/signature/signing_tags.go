package signature

import (
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
)

// List of domain separation tags for protocol signatures.
//
// Protocol-level signature uses BLS signature scheme.
// Each signature involves hashing entity bytes during the
// the hash to curve operation.
// To scope the signature to a specific sub-protocol and simulate multiple
// orthogonal random oracles, the hashing process includes a domain separation specific
// where the signature is used.

// Flow protocol prefix
const protocolPrefix = "FLOW-"

// Flow protocol version
const protocolVersion = "-V00-"

// Ciphersuite index
// Only one ciphersuite is used in Flow protocol
const cipherSuiteIndex = "CS00-"

// an example of domain tag output is :
// FLOW-CERTAIN_DOMAIN-V00-CS00-with-cipherSuite
// where cipherSuite is fixed by the Flow crypto library
// (only one ciphersuite is provided for BLS signatures by the crypto library
// and therefore it's not possible to choose one)
func tag(domain string) string {
	return protocolPrefix + domain + protocolVersion + cipherSuiteIndex + "with-"
}

var (
	// all the tags below are application tags, the crypto library API guarantees
	// that all application tags are different than the tag used to generate
	// proofs of possession of BLS private keys.

	// RandomBeaconTag is used for threshold signatures in the random beacon
	RandomBeaconTag = tag("Random_Beacon")
	// ConsensusVoteTag is used for Consensus Hotstuff votes
	ConsensusVoteTag = tag("Consensus_Vote")
	// CollectorVoteTag is used for Collection Hotstuff votes
	CollectorVoteTag = tag("Collector_Vote")
	// ConsensusTimeoutTag is used for Consensus Active Pacemaker timeouts
	ConsensusTimeoutTag = tag("Consensus_Timeout")
	// CollectorTimeoutTag is used for Collector Active Pacemaker timeouts
	CollectorTimeoutTag = tag("Collector_Timeout")
	// ExecutionReceiptTag is used for execution receipts
	ExecutionReceiptTag = tag("Execution_Receipt")
	// ResultApprovalTag is used for result approvals
	ResultApprovalTag = tag("Result_Approval")
	// SPOCKTag is used to generate SPoCK proofs
	SPOCKTag = tag("SPoCK")
	// DKGMessageTag is used for DKG messages
	DKGMessageTag = tag("DKG_Message")
)

// NewBLSHasher returns a hasher to be used for BLS signing and verifying
// in the protocol and abstracts the hasher details from the protocol logic.
//
// The hasher returned is the the expand-message step in the BLS hash-to-curve.
// It uses a xof (extendable output function) based on KMAC128. It therefore has
// 128-bytes outputs.
func NewBLSHasher(tag string) hash.Hasher {
	return crypto.NewExpandMsgXOFKMAC128(tag)
}
