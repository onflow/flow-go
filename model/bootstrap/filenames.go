package bootstrap

import (
	"path/filepath"
)

// Canonical filenames/paths for bootstrapping files.
var (
	// The Node ID file is used as a helper by the transit scripts
	FilenameNodeID = "node-id"
	PathNodeID     = filepath.Join(DirnamePublicBootstrap, FilenameNodeID)

	// execution state
	DirnameExecutionState = "execution-state"

	// public genesis information
	DirnamePublicBootstrap    = "public-root-information"
	PathInternalNodeInfosPub  = filepath.Join(DirnamePublicBootstrap, "node-internal-infos.pub.json")
	PathFinallist             = filepath.Join(DirnamePublicBootstrap, "finallist.pub.json")
	PathNodeInfosPub          = filepath.Join(DirnamePublicBootstrap, "node-infos.pub.json")
	PathPartnerNodeInfoPrefix = filepath.Join(DirnamePublicBootstrap, "node-info.pub.")
	PathNodeInfoPub           = filepath.Join(DirnamePublicBootstrap, "node-info.pub.%v.json") // %v will be replaced by NodeID
	DirnameRootBlockVotes     = filepath.Join(DirnamePublicBootstrap, "root-block-votes")
	FileNamePartnerWeights    = "partner-weights.json"

	PathRootBlockData             = filepath.Join(DirnamePublicBootstrap, "root-block.json")
	PathRootProtocolStateSnapshot = filepath.Join(DirnamePublicBootstrap, "root-protocol-state-snapshot.json")

	FilenameWALRootCheckpoint = "root.checkpoint"
	PathRootCheckpoint        = filepath.Join(DirnameExecutionState, FilenameWALRootCheckpoint) // only available on an execution node

	// private genesis information
	DirPrivateRoot                   = "private-root-information"
	FilenameRandomBeaconPriv         = "random-beacon.priv.json"
	FilenameSecretsEncryptionKey     = "secretsdb-key"
	PathPrivNodeInfoPrefix           = "node-info.priv."
	FilenameRootBlockVotePrefix      = "root-block-vote."
	PathRootDKGData                  = filepath.Join(DirPrivateRoot, "root-dkg-data.priv.json")
	PathNodeInfoPriv                 = filepath.Join(DirPrivateRoot, "private-node-info_%v", "node-info.priv.json")                 // %v will be replaced by NodeID
	PathNodeMachineAccountPrivateKey = filepath.Join(DirPrivateRoot, "private-node-info_%v", "node-machine-account-key.priv.json")  // %v will be replaced by NodeID
	PathNodeMachineAccountInfoPriv   = filepath.Join(DirPrivateRoot, "private-node-info_%v", "node-machine-account-info.priv.json") // %v will be replaced by NodeID
	PathRandomBeaconPriv             = filepath.Join(DirPrivateRoot, "private-node-info_%v", FilenameRandomBeaconPriv)              // %v will be replaced by NodeID
	PathNodeRootBlockVote            = filepath.Join(DirPrivateRoot, "private-node-info_%v", "root-block-vote.json")
	FilenameRootBlockVote            = FilenameRootBlockVotePrefix + "%v.json"
	PathSecretsEncryptionKey         = filepath.Join(DirPrivateRoot, "private-node-info_%v", FilenameSecretsEncryptionKey) // %v will be replaced by NodeID
)
