package util

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func IsPayloadAtreeInlined(payload *ledger.Payload) (isAtreeSlab bool, isInlined bool, err error) {
	registerID, registerValue, err := convert.PayloadToRegister(payload)
	if err != nil {
		return false, false, fmt.Errorf("failed to convert payload to register: %w", err)
	}
	return IsRegisterAtreeInlined(registerID.Key, registerValue)
}

func IsRegisterAtreeInlined(key string, value []byte) (isAtreeSlab bool, isInlined bool, err error) {
	if !flow.IsSlabIndexKey(key) {
		return false, false, nil
	}

	// Check Atree register version

	head, err := newHeadFromData(value)
	if err != nil {
		return false, false, err
	}

	version := head.version()
	if version > maxSupportedVersion {
		return false, false, fmt.Errorf("atree slab version %d, max supported version %d", version, maxSupportedVersion)
	}

	return true, version == inlinedVersion, nil
}

const (
	maskVersion byte = 0b1111_0000

	noninlinedVersion   = 0
	inlinedVersion      = 1
	maxSupportedVersion = inlinedVersion
)

type head [2]byte

func newHeadFromData(data []byte) (head, error) {
	if len(data) < 2 {
		return head{}, fmt.Errorf("atree slab must be at least 2 bytes, got %d bytes", len(data))
	}

	return head{data[0], data[1]}, nil
}

func (h *head) version() byte {
	return (h[0] & maskVersion) >> 4
}
