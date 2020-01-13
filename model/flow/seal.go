// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

type Seal struct {
}

func (s Seal) ID() Identifier {
	return ZeroID
}

func (s Seal) Checksum() Identifier {
	return ZeroID
}
