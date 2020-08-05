// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

type Seal struct {
	BlockID      Identifier
	ResultID     Identifier
	InitialState StateCommitment
	FinalState   StateCommitment
	//TODO figure out how to represent these, could use flow.Event once format
	// is decided. []interface{} needs extra meta info to send over the wire.
	ServiceEvents []interface{}
}

func (s Seal) Body() interface{} {
	return struct {
		BlockID       Identifier
		ResultID      Identifier
		InitialState  StateCommitment
		FinalState    StateCommitment
		ServiceEvents []interface{}
	}{
		BlockID:       s.BlockID,
		ResultID:      s.ResultID,
		InitialState:  s.InitialState,
		FinalState:    s.FinalState,
		ServiceEvents: s.ServiceEvents,
	}
}

func (s Seal) ID() Identifier {
	return MakeID(s.Body())
}

func (s Seal) Checksum() Identifier {
	return MakeID(s)
}
