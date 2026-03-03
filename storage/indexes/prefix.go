package indexes

const (
	// codeIndexProcessedHeightLowerBound is the prefix for indexer's lower bound of processed heights
	// the second byte is the indexer's prefix code.
	// Example: prefix [8][10] means this entry is the lowest processed height for the type with code 10.
	codeIndexProcessedHeightLowerBound byte = 8
	// codeIndexProcessedHeightUpperBound is the prefix for indexer's upper bound of processed heights
	// the second byte is the indexer's prefix code.
	// Example: prefix [9][10] means this entry is the highest processed height for the type with code 10.
	codeIndexProcessedHeightUpperBound byte = 9

	// Account indexes
	codeAccountTransactions              byte = 10 // Account transactions index
	codeAccountFungibleTokenTransfers    byte = 11 // Account fungible token transfers index
	codeAccountNonFungibleTokenTransfers byte = 12 // Account non-fungible token transfers index

	// reserved as extension byte for future use
	_ byte = 255
)

// Indexer Processed Heights Keys
// these are the currently supported indexers' upper and lower bound height keys
var (
	// Upper and lower bound keys for account transactions
	keyAccountTransactionLatestHeightKey = []byte{codeIndexProcessedHeightUpperBound, codeAccountTransactions}
	keyAccountTransactionFirstHeightKey  = []byte{codeIndexProcessedHeightLowerBound, codeAccountTransactions}

	// Upper and lower bound keys for account fungible token transfers
	keyAccountFTTransferLatestHeightKey = []byte{codeIndexProcessedHeightUpperBound, codeAccountFungibleTokenTransfers}
	keyAccountFTTransferFirstHeightKey  = []byte{codeIndexProcessedHeightLowerBound, codeAccountFungibleTokenTransfers}

	// Upper and lower bound keys for account non-fungible token transfers
	keyAccountNFTTransferLatestHeightKey = []byte{codeIndexProcessedHeightUpperBound, codeAccountNonFungibleTokenTransfers}
	keyAccountNFTTransferFirstHeightKey  = []byte{codeIndexProcessedHeightLowerBound, codeAccountNonFungibleTokenTransfers}
)
