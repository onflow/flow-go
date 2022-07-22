package flow

import (
	"encoding/hex"
	"fmt"

	"testing"
)

// this benchmark can run with this command:
//  go test -run=String -bench=.

// this is to prevent lint errors
var length int

func BenchmarkString(b *testing.B) {

	var r = RegisterID{
		Owner: "theowner",
		Key:   "123412341234",
	}

	ownerLen := len(r.Owner)

	requiredLen := ((ownerLen + len(r.Key)) * 2) + 1

	arr := make([]byte, requiredLen)

	hex.Encode(arr, []byte(r.Owner))

	arr[2*ownerLen] = byte('/')

	hex.Encode(arr[(2*ownerLen)+1:], []byte(r.Key))

	s := string(arr)
	length = len(s)
}

func BenchmarkOriginalString(b *testing.B) {

	var r = RegisterID{
		Owner: "theowner",
		Key:   "123412341234",
	}

	ret := fmt.Sprintf("%x/%x", r.Owner, r.Key)

	length = len(ret)
}
