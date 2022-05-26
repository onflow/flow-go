package state

import (
	"encoding/hex"

	"github.com/onflow/flow-go/fvm/errors"
)

type AccountStatus uint8

const (
	maskExist  byte = 0b0000_0001
	maskFrozen byte = 0b1000_0000
)

// NewAccountStatus sets exist flag and return an AccountStatus
func NewAccountStatus() AccountStatus {
	return AccountStatus(maskExist)
}

func (a AccountStatus) ToBytes() []byte {
	b := make([]byte, 1)
	b[0] = byte(a)
	return b
}

func AccountStatusFromBytes(inp []byte) (AccountStatus, error) {
	// if len of inp is zero, account does not exist
	if len(inp) == 0 {
		return 0, nil
	}
	if len(inp) > 1 {
		return 0, errors.NewValueErrorf(hex.EncodeToString(inp), "invalid account state")
	}
	return AccountStatus(inp[0]), nil
}

func (a AccountStatus) AccountExists() bool {
	return a > 0
}

func (a AccountStatus) IsAccountFrozen() bool {
	return uint8(a)&maskFrozen > 0
}

func SetAccountStatusFrozenFlag(inp AccountStatus, frozen bool) AccountStatus {
	if frozen {
		return AccountStatus(uint8(inp) | maskFrozen)
	}
	return AccountStatus(uint8(inp) & (0xFF - maskFrozen))
}
