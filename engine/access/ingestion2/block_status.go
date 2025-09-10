package ingestion2

// BlockStatus represents the status of a block in the forest.
type BlockStatus uint64

const (
	// BlockStatusCertified indicates the block is certified
	BlockStatusCertified BlockStatus = iota + 1
	// BlockStatusFinalized indicates the block is finalized
	BlockStatusFinalized
	// BlockStatusSealed indicates the block has been sealed
	BlockStatusSealed
)

// String returns the string representation of the block status
func (bs BlockStatus) String() string {
	switch bs {
	case BlockStatusCertified:
		return "certified"
	case BlockStatusFinalized:
		return "finalized"
	case BlockStatusSealed:
		return "sealed"
	default:
		return "unknown"
	}
}

// IsValid returns true if the block status is a valid value.
func (bs BlockStatus) IsValid() bool {
	switch bs {
	case BlockStatusCertified, BlockStatusFinalized, BlockStatusSealed:
		return true
	default:
		return false
	}
}
