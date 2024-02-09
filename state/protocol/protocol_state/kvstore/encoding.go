package kvstore

import (
	"bytes"
	"fmt"
	"github.com/vmihailenco/msgpack/v4"
)

func versionedEncode(version uint64, pairs any) (uint64, []byte, error) {
	bz, err := msgpack.Marshal(pairs)
	if err != nil {
		return 0, nil, fmt.Errorf("could not encode kvstore (version=%d): %w", version, err)
	}
	return version, bz, nil
}

func Decode(version uint64, bz []byte) (LatestKVApi, error) {
	var target any
	switch version {
	case 0:
		target = KVPairsV0{}
	case 1:
		target = KVPairsV1{}
	}
	err := msgpack.NewDecoder(bytes.NewBuffer(bz)).Decode(&target)
}
