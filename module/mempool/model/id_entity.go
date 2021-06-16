package model

import (
	"os"
	"strconv"

	"github.com/aristanetworks/goarista/monotime"

	"github.com/onflow/flow-go/model/flow"
)

// IdEntity is an internal data structure for mempool
// It implements a wrapper around the original flow Identifier
// which represents it as a flow Entity
// that allows the identifier to directly gets stored in the mempool
type IdEntity struct {
	Id flow.Identifier
}

// ID implements flow.Entity.ID for Identifier to make it capable of being stored directly
// in mempools and storage
// ID returns the identifier itself
func (id IdEntity) ID() flow.Identifier {
	return id.Id
}

var logfile_identity *os.File

// ID implements flow.Entity.ID for Identifier to make it capable of being stored directly
// in mempools and storage
// ID returns checksum of identifier
func (id IdEntity) Checksum() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/module/mempool/model/id_entity.log")
		logfile_identity = newfile
	})
	ts := monotime.Now()
	defer logfile_identity.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(id)
}
