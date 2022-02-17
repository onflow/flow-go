package ledger

import (
	"fmt"
	"strconv"
	"testing"
)

// this benchmark can run with this command:
//  go test -run=CanonicalForm -bench=.

//nolint
func BenchmarkCanonicalForm(b *testing.B) {

	constant := 10

	keyParts := make([]KeyPart, 0, 200)

	for i := 0; i < 16; i++ {
		kp := NewKeyPart(1234, []byte("somedomain1"))
		keyParts = append(keyParts, kp)
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
		kp := NewKeyPart(1234, []byte("somedomain1"))
		keyParts = append(keyParts, kp)
	}

	ret := ""

	for _, kp := range keyParts {
		ret += fmt.Sprintf("/%d/%v", kp.Type, string(kp.Value))
	}
}
