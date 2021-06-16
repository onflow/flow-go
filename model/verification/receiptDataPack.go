package verification

import (
	"os"
	"strconv"
	"sync"

	"context"

	"github.com/aristanetworks/goarista/monotime"

	"github.com/onflow/flow-go/model/flow"
)

// ReceiptDataPack represents an execution receipt with some metadata.
// This is an internal entity for verification node.
type ReceiptDataPack struct {
	Receipt  *flow.ExecutionReceipt
	OriginID flow.Identifier
	Ctx      context.Context // used for span tracing
}

// ID returns the unique identifier for the ReceiptDataPack which is the
// id of its execution receipt.
func (r *ReceiptDataPack) ID() flow.Identifier {
	return r.Receipt.ID()
}

var once sync.Once
var logfile_rcp_data *os.File

// Checksum returns the checksum of the ReceiptDataPack.
func (r *ReceiptDataPack) Checksum() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/model/verification/receiptdatapack.log")
		logfile_rcp_data = newfile
	})
	ts := monotime.Now()

	defer logfile_rcp_data.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(r)
}
