package encoding

// List of string constants, aka 'tags' for signature typing
// Each Protocol-level signature, essentially signs the hash of an entity. To prevent
// type-malleability attacks, the signed data must also include a tag which specifies the
// type of the hashed-and-signed object.

const (
	RandomBeaconTag     = "RandomBeacon"
	ConsensusVoteTag    = "ConsensusVote"
	CollectorVoteTag    = "CollectorVote"
	ExecutionReceiptTag = "ExecutionReceipt"
	SPoCKTag            = "SPoCK"
	ResultApprovalTag   = "ResultApproval"
)
