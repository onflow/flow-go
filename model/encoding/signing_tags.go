package encoding

// List of domain separation tags for protocol signatures.
//
// Each Protocol-level signature involves hashing an entity.
// To prevent domain malleability attacks and to simulate multiple
// independent random oracles, the hashing process includes
// a domain tag that specifies the type of the signed object.

const (
	// Flow protocol version and prefix
	versionPrefix = "FLOW-V0.0-"
	// RandomBeaconTag is used for threshold signatures in the random beacon
	RandomBeaconTag = versionPrefix + "RandomBeacon"
	// ConsensusVoteTag is used for Consensus Hotstuff votes
	ConsensusVoteTag = versionPrefix + "ConsensusVote"
	// CollectorVoteTag is used for Collection Hotstuff votes
	CollectorVoteTag = versionPrefix + "CollectorVote"
	// ExecutionReceiptTag is used for execution receipts
	ExecutionReceiptTag = versionPrefix + "ExecutionReceipt"
	// ResultApprovalTag is used for result approvals
	ResultApprovalTag = versionPrefix + "ResultApproval"
	// SPOCKTag is used to generate SPoCK proofs
	SPOCKTag = versionPrefix + "SPoCK"
)
