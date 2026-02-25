package access

import (
	"math/big"

	"github.com/onflow/flow-go/model/flow"
)

type TransferRole string

const (
	TransferRoleSender    TransferRole = "sender"
	TransferRoleRecipient TransferRole = "recipient"
)

// FungibleTokenTransfer represents a fungible token transfer event extracted from block execution data.
// Each transfer is identified by its position within the block (TransactionIndex, EventIndex).
type FungibleTokenTransfer struct {
	TransactionID    flow.Identifier // Transaction that produced the transfer event
	BlockHeight      uint64          // Block height where the transaction was included
	BlockTimestamp   uint64          // Block timestamp where the transaction was included
	TransactionIndex uint32          // Index of the transaction within the block
	EventIndices     []uint32        // Index of the event within the transaction
	TokenType        string          // Fully qualified token type identifier (e.g., "A.0x1654653399040a61.FlowToken")
	Amount           *big.Int        // Amount of tokens transferred
	SourceAddress    flow.Address    // Account that sent the tokens
	RecipientAddress flow.Address    // Account that received the tokens

	// Expansion fields populated when expandResults is true.
	Transaction *flow.TransactionBody // Transaction body (nil unless expanded)
	Result      *TransactionResult    // Transaction result (nil unless expanded)
}

// NonFungibleTokenTransfer represents a non-fungible token transfer event extracted from block execution data.
// Each transfer is identified by its position within the block (TransactionIndex, EventIndex).
type NonFungibleTokenTransfer struct {
	TransactionID    flow.Identifier // Transaction that produced the transfer event
	BlockHeight      uint64          // Block height where the transaction was included
	BlockTimestamp   uint64          // Block timestamp where the transaction was included
	TransactionIndex uint32          // Index of the transaction within the block
	EventIndices     []uint32        // Index of the event within the transaction
	TokenType        string          // Fully qualified type of NFT collection (e.g., "A.0x1654653399040a61.MyNFT")
	ID               uint64          // Unique identifier of the NFT within its collection
	SourceAddress    flow.Address    // Account that sent the token
	RecipientAddress flow.Address    // Account that received the token

	// Expansion fields populated when expandResults is true.
	Transaction *flow.TransactionBody // Transaction body (nil unless expanded)
	Result      *TransactionResult    // Transaction result (nil unless expanded)
}

// TransferCursor identifies a position in the token transfer index for cursor-based pagination.
// It corresponds to the last entry returned in a previous page.
type TransferCursor struct {
	Address          flow.Address // Account address
	BlockHeight      uint64       // Block height of the last returned entry
	TransactionIndex uint32       // Transaction index within the block of the last returned entry
	EventIndex       uint32       // Event index within the transaction of the last returned entry
}

// FungibleTokenTransfersPage represents a single page of fungible token transfer results.
type FungibleTokenTransfersPage struct {
	Transfers  []FungibleTokenTransfer // Transfers in this page (descending order by height)
	NextCursor *TransferCursor         // Cursor to fetch the next page, nil when no more results
}

// NonFungibleTokenTransfersPage represents a single page of non-fungible token transfer results.
type NonFungibleTokenTransfersPage struct {
	Transfers  []NonFungibleTokenTransfer // Transfers in this page (descending order by height)
	NextCursor *TransferCursor            // Cursor to fetch the next page, nil when no more results
}
