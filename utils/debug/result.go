package debug

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/onflow/flow-go/model/flow"
)

func WriteResult(w io.Writer, id flow.Identifier, result Result) {
	_, _ = fmt.Fprintf(w, "# ID: %s\n", id)

	_, _ = fmt.Fprintf(w, "# Success: %v\n", result.Output.Err == nil)

	_, _ = fmt.Fprintf(w, "# Events:\n")
	for _, event := range result.Output.Events {
		_, _ = fmt.Fprintf(w, "- %s\n", event)
	}

	_, _ = fmt.Fprintf(w, "# Logs:\n")
	for _, log := range result.Output.Logs {
		_, _ = fmt.Fprintf(w, "- %q\n", log)
	}

	_, _ = fmt.Fprintf(w, "# Read registers:\n")
	readRegisterIDs := result.Snapshot.ReadRegisterIDs()
	sortRegisterIDs(readRegisterIDs)
	for _, readRegisterID := range readRegisterIDs {
		_, _ = fmt.Fprintf(w, "%s\n", readRegisterID)
	}

	_, _ = fmt.Fprintf(w, "# Updated registers:\n")
	updatedRegisters := result.Snapshot.UpdatedRegisters()
	sortRegisterEntries(updatedRegisters)
	for _, updatedRegister := range updatedRegisters {
		_, _ = fmt.Fprintf(
			w,
			"%s = %s\n",
			updatedRegister.Key,
			hex.EncodeToString(updatedRegister.Value),
		)
	}
}
