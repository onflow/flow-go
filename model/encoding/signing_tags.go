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

// Flow protocol version and prefix
const protocolPrefix = "FLOW-V0.0_"

var (
	// all the tags below are application tags, the crypto library API guarantees
	// that all application tags are different than the tag used to generate
	// proofs of possession of BLS private keys.

	// RandomBeaconTag is used for threshold signatures in the random beacon
	RandomBeaconTag = tag("Random-Beacon")
	// ConsensusVoteTag is used for Consensus Hotstuff votes
	ConsensusVoteTag = tag("Consensus-Vote")
	// CollectorVoteTag is used for Collection Hotstuff votes
	CollectorVoteTag = tag("Collector-Vote")
	// ExecutionReceiptTag is used for execution receipts
	ExecutionReceiptTag = tag("Execution-Receipt")
	// ResultApprovalTag is used for result approvals
	ResultApprovalTag = tag("Result-Approval")
	// SPOCKTag is used to generate SPoCK proofs
	SPOCKTag = tag("SPoCK")
	// DKGMessageTag is used for DKG messages
	DKGMessageTag = tag("DKG-Message")
)
