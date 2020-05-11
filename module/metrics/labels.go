package metrics

const (
	LabelChannel  = "topic"
	LabelChain    = "chain"
	EngineLabel   = "engine"
	LabelResource = "resource"
	LabelMessage  = "message"
)

const (
	ChannelNone = "none"
)

const (
	EnginePropagation     = "propagation"
	EngineCompliance      = "compliance"
	EngineProvider        = "provider"
	EngineIngestion       = "ingestion"
	EngineMatching        = "matching"
	EngineSynchronization = "sync"
)

const (
	ResourceUndefined = "undefined"
	ResourceProposal  = "proposal"
	ResourceHeader    = "header"
	ResourceIndex     = "index"
	ResourceIdentity  = "identity"
	ResourceGuarantee = "guarantee"
	ResourceReceipt   = "receipt"
	ResourceApproval  = "approval"
	ResourceSeal      = "seal"
)

const (
	MessageCollectionGuarantee = "guarantee"
	MessageBlockProposal       = "proposal"
	MessageBlockVote           = "vote"
	MessageExecutionReceipt    = "receipt"
	MessageResultApproval      = "approval"
	MessageSyncRequest         = "ping"
	MessageSyncResponse        = "pong"
	MessageRangeRequest        = "range"
	MessageBatchRequest        = "batch"
	MessageBlockResponse       = "block"
)
