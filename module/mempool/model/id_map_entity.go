package model

import (
	"os"
	"strconv"

	"github.com/aristanetworks/goarista/monotime"

	"github.com/onflow/flow-go/model/flow"
)

// IdMapEntity is an internal data structure for mempool
// It implements a key-value entry where an identifier is mapped to a list of other identifiers.
type IdMapEntity struct {
	Key flow.Identifier
	IDs map[flow.Identifier]struct{}
}

// ID implements flow.Entity.ID for IdMapEntity to make it capable of being stored directly
// in mempools and storage. It returns key field of the id.
func (id IdMapEntity) ID() flow.Identifier {
	return id.Key
}

var logfile_mapentity *os.File

// CheckSum implements flow.Entity.CheckSum for IdMapEntity to make it capable of being stored directly
// in mempools and storage. It makes the id of the entire IdMapEntity.
func (id IdMapEntity) Checksum() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/module/mempool/id_map_entity.log")
		logfile_mapentity = newfile
	})
	ts := monotime.Now()
	defer logfile_mapentity.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(id)
}
