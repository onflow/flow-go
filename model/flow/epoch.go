package flow

type EpochSetup struct {
	Counter    uint64
	FinalView  uint64
	Identities IdentityList
	Clusters   *ClusterList
	Seed       []byte
}

// TODO: the usage of flow events inside of the main repo
// creates significant issues around dependencies for
// transactions events, which are mostly located in the
// SDK repo for now.
type EpochCommit struct {
	Counter uint64
	// DKGData    *dkg.PublicData
	// ClusterQCs []*model.QuorumCertificate
}
