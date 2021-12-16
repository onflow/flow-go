package response

import (
	"encoding/base64"
	"fmt"
)

func fromUint64(number uint64) string {
	return fmt.Sprintf("%d", number)
}

func toBase64(byteValue []byte) string {
	return base64.StdEncoding.EncodeToString(byteValue)
}
