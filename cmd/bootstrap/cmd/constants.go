package cmd

const (
	randomSeedBytes           = 64
	minSeedBytes              = 64
	minNodesPerCluster uint16 = 3

	dirnameExecutionState         = "execution-state"
	filenameGenesisBlock          = "genesis-block.json"
	filenameNodeInfosPub          = "node-infos.pub.json"
	filenameNodeInfoPriv          = "%v.node-info.priv.json"     // %v will be replaced by NodeID
	filenameNodeInfoPub           = "%v.node-info.pub.json"      // %v will be replaced by NodeID
	filenamePartnerNodeInfoSuffix = ".node-info.pub.json"        // %v will be replaced by NodeID
	filenameRandomBeaconPriv      = "%v.random-beacon.priv.json" // %v will be replaced by NodeID
	filenameDKGDataPub            = "dkg-data.pub.json"
	filenameAccount0Priv          = "account-0.priv.json"
	filenameGenesisQC             = "genesis-qc.json"
)
