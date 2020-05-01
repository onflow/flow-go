package metrics

// Prometheus metric namespaces
const (
	namespaceCommon       = "common"
	namespaceCollection   = "collection"
	namespaceConsensus    = "consensus"
	namespaceVerification = "verification"
	namespaceExecution    = "execution"
)

// Prometheus metric subsystems
const (
	subsystemBadger  = "badger"
	subsystemNetwork = "network"

)

// Execution Subsystems
const (
	subsystemStateStorage = "state_storage"
	subsystemRuntime      = "runtime"
)
