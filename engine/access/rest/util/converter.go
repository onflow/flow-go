package util

import (
	"encoding/base64"
	"fmt"
	"strconv"
)

// FromUint convert uint to string
func FromUint[U uint | uint64 | uint32](number U) string {
	return fmt.Sprintf("%d", number)
}

// ToUint64 convert input string to uint64 number
func ToUint64(uint64Str string) (uint64, error) {
	val, err := strconv.ParseUint(uint64Str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("value must be an unsigned 64 bit integer") // hide error from user
	}
	return val, nil
}

// ToUint32 convert input string to uint64 number
func ToUint32(uint32Str string) (uint32, error) {
	val, err := strconv.ParseUint(uint32Str, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("value must be an unsigned 32 bit integer") // hide error from user
	}
	return uint32(val), nil
}

// ToBase64 converts byte input to string base64 encoded output
func ToBase64(byteValue []byte) string {
	return base64.StdEncoding.EncodeToString(byteValue)
}

// FromBase64 convert input base64 encoded string to decoded bytes
func FromBase64(bytesStr string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(bytesStr)
}
