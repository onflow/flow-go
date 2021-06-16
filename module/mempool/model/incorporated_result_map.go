package model


import (
	"os"
	"strconv"

	"github.com/aristanetworks/goarista/monotime"

	"github.com/onflow/flow-go/model/flow"
)

// IncorporatedResultMap is an internal data structure for the incorporated
// results mempool. IncorporatedResults are indexed by ExecutionResult ID and
// IncorporatedBlockID
type IncorporatedResultMap struct {
	ExecutionResult     *flow.ExecutionResult
	IncorporatedResults map[flow.Identifier]*flow.IncorporatedResult // [incorporated block ID] => IncorporatedResult
}

// ID implements flow.Entity.ID for IncorporatedResultMap to make it capable of
// being stored directly in mempools and storage.
func (a *IncorporatedResultMap) ID() flow.Identifier {
	return a.ExecutionResult.ID()
}

var logfile_inc_result *os.File

// CheckSum implements flow.Entity.CheckSum for IncorporatedResultMap to make it
// capable of being stored directly in mempools and storage. It makes the id of
// the entire IncorporatedResultMap.
func (a *IncorporatedResultMap) Checksum() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/module/mempool/model/incorporated_result_map.log")
		logfile_inc_result = newfile
	})
	ts := monotime.Now()
	defer logfile_inc_result.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(a)
}
