package factories

//
//type BuilderFactory interface {
//	Create(
//		headers storage.Headers,
//		payloads storage.ClusterPayloads,
//		pool mempool.Transactions,
//	) (module.BuilderFactory, module.Finalizer, error)
//}
//
//type ClusterStateFactory interface {
//	Create(
//		clusterID flow.ChainID,
//	) (
//		cluster.State,
//		storage.Headers,
//		storage.ClusterPayloads,
//		storage.ClusterBlocks,
//		error,
//	)
//}
//
//type HotStuffFactory interface {
//	Create(
//		clusterID flow.ChainID,
//		cluster flow.IdentityList,
//		clusterState cluster.State,
//		headers storage.Headers,
//		payloads storage.ClusterPayloads,
//		seed []byte,
//		builder module.BuilderFactory,
//		updater module.Finalizer,
//		communicator hotstuff.Communicator,
//		rootHeader *flow.Header,
//		rootQC *flow.QuorumCertificate,
//	) (module.HotStuff, error)
//}
//
//type ProposalEngineFactory interface {
//	Create(
//		state cluster.State,
//		headers storage.Headers,
//		payloads storage.ClusterPayloads,
//	) (module.ReadyDoneAware, error)
//}
//
//type SyncEngineFactory interface {
//	Create(
//		participants flow.IdentityList,
//		state cluster.State,
//		blocks storage.ClusterBlocks,
//		comp network.Engine,
//	) (module.SyncCore, module.ReadyDoneAware, error)
//}
