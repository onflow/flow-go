package types

// SignerIndex is the index of the node participated hotstuff consensus in a group.
// The purpose of SignerIndex in a signature is to identify the signer.
// It's a more compressed form than public key to identify a node.
// A signer's identify can be found by specifiying the following three information:
//   (SignerIndex, IdentifierFilter, BlockID).
// a uint32 value is enough to cover the possible range.
type SignerIndex uint32

// SignerIndexes is the compressed form of a list of SignerIndex
// Currently it's implemented as []bool, but more compressed form like bitset
// could be explored.
type SignerIndexes []bool
