package model

import (
	"os"
	"strconv"
	"sync"

	"github.com/aristanetworks/goarista/monotime"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// Vote is the HotStuff algorithm's concept of a vote for a block proposal.
type Vote struct {
	View     uint64
	BlockID  flow.Identifier
	SignerID flow.Identifier
	SigData  []byte
}

var once sync.Once
var logfile_vote *os.File

// ID returns the identifier for the vote.
func (uv *Vote) ID() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/consensus/hotstuff/model/vote.log")
		logfile_vote = newfile
	})
	ts := monotime.Now()
	defer logfile_vote.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(uv)
}

// VoteFromFlow turns the vote parameters into a vote struct.
func VoteFromFlow(signerID flow.Identifier, blockID flow.Identifier, view uint64, sig crypto.Signature) *Vote {
	vote := Vote{
		View:     view,
		BlockID:  blockID,
		SignerID: signerID,
		SigData:  sig,
	}
	return &vote
}
