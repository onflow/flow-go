package metrics

// Prometheus metric namespaces
const (
	namespaceCommon       = "common"
	namespaceCollection   = "collection"
	namespaceConsensus    = "consensus"
	namespaceVerification = "verification"
)

// Prometheus metric subsystems
const (
	subsystemBadger     = "badger"
	subsystemProposal   = "proposal"
	subsystemCompliance = "compliance"
	subsystemNetwork    = "network"
)
