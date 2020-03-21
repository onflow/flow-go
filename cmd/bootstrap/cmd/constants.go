package cmd

const (
	randomSeedBytes = 64
	minSeedBytes    = 64

	filenameGenesisBlock          = "genesis-block.json"
	filenameNodeInfosPub          = "node-infos.pub.json"
	filenameRandomBeaconPriv      = "%v.random-beacon.priv.json" // %v will be replaced by NodeID
	filenameDKGDataPub            = "dkg-data.pub.json"
	filenameNodeInfoPriv          = "%v.node-info.priv.json" // %v will be replaced by NodeID
	filenameNodeInfoPub           = "%v.node-info.pub.json"  // %v will be replaced by NodeID
	filenamePartnerNodeInfoSuffix = ".node-info.pub.json"    // %v will be replaced by NodeID
	filenameGenesisQC             = "genesis-qc.json"
	filenameAccount0Priv          = "account-0.priv.json"
)
