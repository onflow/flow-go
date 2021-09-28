package migrations

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
)

// reports on the state of the registers
type RegisterReporter struct {
	Log       zerolog.Logger
	OutputDir string
}

func (r RegisterReporter) filename() string {
	return path.Join(r.OutputDir, fmt.Sprintf("register_report_%d.csv", int32(time.Now().Unix())))
}

func (r RegisterReporter) Report(payload []ledger.Payload) error {
	fn := r.filename()
	r.Log.Info().Msgf("Running Register Reporter. Saving output to %s.", fn)

	f, err := os.Create(fn)
	if err != nil {
		return err
	}

	defer func() {
		err = f.Close()
		if err != nil {
			panic(err)
		}
	}()

	writer := bufio.NewWriter(f)
	defer func() {
		err = writer.Flush()
		if err != nil {
			panic(err)
		}
	}()

	fvm, storage, slabs := splitPayloads(payload)

	_, err = writer.WriteString(fmt.Sprintf("fvm registers: %d, storage registers: %d, slab registers: %d \n", len(fvm), len(storage), len(slabs)))
	if err != nil {
		return err
	}

	r.Log.Info().Msg("Register Reporter Done.")
	return nil
}

// func extractSubtreeSlabIDs(inp *ledger.Payload) {

// 	flag := inp.Value[0]

// 	dataType := (flag & byte(0b000_11000)) >> 3
// 	switch dataType {
// 	case 0: // slab Array

// 	case 1: // slab Map

// 	case 3: // slabStorable

// 		atree.DecodeStorageIDStorable()
// 	default:
// 		// error out
// 		panic("unknown type of slab")
// 	}

// }
