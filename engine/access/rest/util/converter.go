package util

import (
	"encoding/base64"
	"fmt"
	"strconv"
)

// FromUint64 convert uint64 to string
func FromUint64(number uint64) string {
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

// ToBase64 converts byte input to string base64 encoded output
func ToBase64(byteValue []byte) string {
	return base64.StdEncoding.EncodeToString(byteValue)
}

// FromBase64 convert input base64 encoded string to decoded bytes
func FromBase64(bytesStr string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(bytesStr)
}
