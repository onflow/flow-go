package encoding

// List of domain separation tags for protocol signatures.
//
// Each Protocol-level signature involves hashing an entity.
// To prevent domain malleability attacks and to simulate multiple
// orthogonal random oracles, the hashing process includes
// a domain tag that specifies the type of the signed object.

func tag(domain string) string {
	return protocolPrefix + domain
}

// Flow protocol version and ciphersuite
const protocolPrefix = "FLOW-V00-CS00-with-"

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
