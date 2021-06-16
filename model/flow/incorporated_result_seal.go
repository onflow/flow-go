package flow

import (
	"strconv"
	"os"

	"github.com/aristanetworks/goarista/monotime"

)

// IncorporatedResultSeal is a wrapper around a seal that keeps track of which
// IncorporatedResult the seal corresponds to. Sealing is a function of result
// And the ID of the block in which the result was incorporated, which is all
// contained in IncorporatedResult.
type IncorporatedResultSeal struct {
	// IncorporatedResult is the incorporated result (result + ID of block where
	// it was incorporated) that the seal is for.
	IncorporatedResult *IncorporatedResult

	// Seal is a seal for the result contained in IncorporatedResult.
	Seal *Seal

	// the header of the executed block
	// useful for indexing the seal by height in the mempool in order for fast pruning
	Header *Header
}

// ID implements flow.Entity.ID for IncorporatedResultSeal to make it capable of
// being stored directly in mempools and storage.
func (s *IncorporatedResultSeal) ID() Identifier {
	return s.IncorporatedResult.ID()
}

var logfile_inc_seal *os.File

// CheckSum implements flow.Entity.CheckSum for IncorporatedResultSeal to make
// it capable of being stored directly in mempools and storage.
func (s *IncorporatedResultSeal) Checksum() Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/model/flow/incorporated_result_seal.log")

		logfile_inc_seal = newfile
	})

	ts := monotime.Now()

	defer logfile_inc_seal.WriteString(strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return MakeID(s)
}
