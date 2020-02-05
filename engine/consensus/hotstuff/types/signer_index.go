package types

// SignerIndex is the index of the node participated in a cluster for hotstuff consensus.
// The purpose of SignerIndex is to identify the signer when included in an aggregated
// signature. It's a more compressed form than public key to identify a node.
// A slice of bytes is used in order to abstract the implementation detail.
type SignerIndex []byte
