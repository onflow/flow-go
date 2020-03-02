package trie

var defaultLeafHash []byte = Hash(EmptySlice)
var defaultHashes [257][]byte // The zero hashes of the level of the tree

func init() {
	// Creates the Default hashes from base to level height
	defaultHashes[0] = defaultLeafHash
	for i := 1; i < len(defaultHashes); i++ {
		defaultHashes[i] = Hash(defaultHashes[i-1], defaultHashes[i-1])
	}
}

// GetDefaultHashes returns the default hashes of the SMT.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashes() [257][]byte {
	return defaultHashes
}

// GetDefaultHashForHeight returns the default hashes of the SMT at a specified height.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashForHeight(height int) []byte {
	return defaultHashes[height]
}
