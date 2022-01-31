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
		Owner:      "theowner",
		Controller: "thecontroller",
		Key:        "123412341234",
	}

	ownerLen := len(r.Owner)
	controllerLen := len(r.Controller)

	requiredLen := ((ownerLen + controllerLen + len(r.Key)) * 2) + 2

	arr := make([]byte, requiredLen)

	hex.Encode(arr, []byte(r.Owner))

	arr[2*ownerLen] = byte('/')

	hex.Encode(arr[(2*ownerLen)+1:], []byte(r.Controller))

	arr[2*(ownerLen+controllerLen)+1] = byte('/')

	hex.Encode(arr[2*(ownerLen+controllerLen+1):], []byte(r.Key))

	s := string(arr)
	length = len(s)
}

func BenchmarkOriginalString(b *testing.B) {

	var r = RegisterID{
		Owner:      "theowner",
		Controller: "thecontroller",
		Key:        "123412341234",
	}

	ret := fmt.Sprintf("%x/%x/%x", r.Owner, r.Controller, r.Key)

	length = len(ret)
}
