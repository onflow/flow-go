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

	// codeAccountTransactions is the prefix for account transaction index entries
	codeAccountTransactions byte = 10

	// reserved as extension byte for future use
	_ byte = 255
)

// Indexer Processed Heights Keys
// these are the currently supported indexers' upper and lower bound height keys
var (
	// keyAccountTransactionLatestHeightKey stores the latest indexed height for account transactions
	keyAccountTransactionLatestHeightKey = []byte{codeIndexProcessedHeightUpperBound, codeAccountTransactions}
	// keyAccountTransactionFirstHeightKey stores the first indexed height for account transactions
	keyAccountTransactionFirstHeightKey = []byte{codeIndexProcessedHeightLowerBound, codeAccountTransactions}
)
