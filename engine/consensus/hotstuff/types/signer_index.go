package types

// SignerIndex is the index of the node participated hotstuff consensus in a group.
// The purpose of SignerIndex in a signature is to identify the signer.
// It's a more compressed form than public key to identify a node.
// A signer's identify can be found by specifiying the following three information:
//   (SignerIndex, IdentifierFilter, BlockID).
// A slice of bytes is used in order to abstract the implementation detail.
type SignerIndex []byte

// SignerIndexes is the compressed form of a list of SignerIndex
type SignerIndexes []byte
