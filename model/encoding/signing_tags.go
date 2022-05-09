package encoding

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

// version and ciphersuite index
const version = "-V00-CS00-with-"

// an example of domain tag output is :
// FLOW-CERTAIN_DOMAIN-V00-CS00-with-
// The
func tag(domain string) string {
	return protocolPrefix + domain + version
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
	// ExecutionReceiptTag is used for execution receipts
	ExecutionReceiptTag = tag("Execution_Receipt")
	// ResultApprovalTag is used for result approvals
	ResultApprovalTag = tag("Result_Approval")
	// SPOCKTag is used to generate SPoCK proofs
	SPOCKTag = tag("SPoCK")
	// DKGMessageTag is used for DKG messages
	DKGMessageTag = tag("DKG_Message")
)
