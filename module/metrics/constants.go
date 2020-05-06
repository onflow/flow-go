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
	subsystemBadger     = "badger"
	subsystemProposal   = "proposal"
	subsystemCompliance = "compliance"
	subsystemNetwork    = "network"
)

// Execution Subsystems
const (
	subsystemStateStorage = "state_storage"
	subsystemRuntime      = "runtime"
)

// METRIC NAMING GUIDELINES
// Namespace:
//   * If it's under a module, use the module name. eg: hotstuff, network, storage, mempool, interpreter, crypto
//   * If it's a core metric from a node, use the node type. eg: consensus, verification, access
// Subsystem:
//   * Subsystem is optional if the entire namespace is small enough to not be segmented further.
//   * Within the component, describe the part or function referred to.
// Constant Labels:
//    * node_role: [collection, consensus, execution, verification, access]
//    * beta_metric: true

// Namespaces for HotStuff:
const (
	hotStuffModuleNamespace      = "hotstuff"
	hotStuffParticipantSubsystem = "Participant"
	hotStuffFollowerSubsystem    = "Follower" // will be used later
)
