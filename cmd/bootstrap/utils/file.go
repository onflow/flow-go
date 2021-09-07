package utils

import (
	"fmt"
	"path/filepath"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/state/protocol/inmem"
	io "github.com/onflow/flow-go/utils/io"
)

func ReadRootProtocolSnapshot(bootDir string) (*inmem.Snapshot, error) {

	// default path root snapshot is written to
	snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)

	// read snapshot file bytes
	bz, err := io.ReadFile(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("could not read snapshot: %w", err)
	}

	// convert bytes to snapshot struct
	snapshot, err := convert.BytesToInmemSnapshot(bz)
	if err != nil {
		return nil, fmt.Errorf("could not convert bytes to snapshot: %w", err)
	}

	return snapshot, nil
}
