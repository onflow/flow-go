package encoding

// List of domain separation tags for protocol signatures.
//
// Each Protocol-level signature involves hashing an entity.
// To prevent domain malleability attacks and to simulate multiple
// independent random oracles, the hashing process includes
// a domain tag that specifies the type of the signed object.

const (
	// RandomBeaconTag is used for threshold signatures in the random beacon
	RandomBeaconTag = "FLOW-V0.0-RandomBeacon"
	// ConsensusVoteTag is used for Consensus Hotstuff votes
	ConsensusVoteTag = "FLOW-V0.0-ConsensusVote"
	// CollectorVoteTag is used for Collection Hotstuff votes
	CollectorVoteTag = "FLOW-V0.0-CollectorVote"
	// ExecutionReceiptTag is used for execution receipts
	ExecutionReceiptTag = "FLOW-V0.0-ExecutionReceipt"
	// ResultApprovalTag is used for result approvals
	ResultApprovalTag = "FLOW-V0.0-ResultApproval"
)
