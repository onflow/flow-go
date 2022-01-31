package ledger

import (
	"fmt"
	"strconv"
	"testing"
)

// this benchmark can run with this command:
//  go test -run=CanonicalForm -bench=.

func BenchmarkCanonicalForm(b *testing.B) {

	constant := 10

	keyParts := make([]KeyPart, 0, 200)

	for i := 0; i < 16; i++ {
		keyParts = append(keyParts, KeyPart{})
		keyParts[i].Value = []byte("somedomain1")
		keyParts[i].Type = 1234
	}

	requiredLen := constant * len(keyParts)
	for _, kp := range keyParts {
		requiredLen += len(kp.Value)
	}

	retval := make([]byte, 0, requiredLen)

	for _, kp := range keyParts {
		typeNumber := strconv.Itoa(int(kp.Type))

		retval = append(retval, byte('/'))
		retval = append(retval, []byte(typeNumber)...)
		retval = append(retval, byte('/'))
		retval = append(retval, kp.Value...)
	}
}

func BenchmarkOriginalCanonicalForm(b *testing.B) {
	keyParts := make([]KeyPart, 0, 200)

	for i := 0; i < 16; i++ {
		keyParts = append(keyParts, KeyPart{})
		keyParts[i].Value = []byte("somedomain1")
		keyParts[i].Type = 1234
	}

	ret := ""

	for _, kp := range keyParts {
		ret += fmt.Sprintf("/%d/%v", kp.Type, string(kp.Value))
	}
}
