package chunks

import (
	"os"
	"strconv"
	"sync"

	"github.com/aristanetworks/goarista/monotime"

	"github.com/onflow/flow-go/model/flow"
)

// Locator is used to locate a chunk by providing the execution result the chunk belongs to as well as the chunk index within that execution result.
// Since a chunk is unique by the result ID and its index in the result's chunk list.
type Locator struct {
	ResultID flow.Identifier // execution result id that chunk belongs to
	Index    uint64          // index of chunk in the execution result
}

var once sync.Once
var logfile_chunklocat_id *os.File

// ID returns a unique id for chunk locator.
func (c Locator) ID() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/model/chunks/chunklocator_id.log")
		logfile_chunklocat_id = newfile
	})
	ts := monotime.Now()
	defer logfile_chunklocat_id.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(c)
}

var logfile_chunklocat_chk * os.File

// Checksum provides a cryptographic commitment for a chunk locator content.
func (c Locator) Checksum() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/model/chunks/chunklocator_chk.log")
		logfile_chunklocat_chk = newfile
	})
	ts := monotime.Now()
	defer logfile_chunklocat_chk.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(c)
}

type LocatorList []*Locator
