package utils

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
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

func ReadRootBlock(rootBlockDataPath string) (*flow.Block, error) {
	bytes, err := io.ReadFile(rootBlockDataPath)
	if err != nil {
		return nil, fmt.Errorf("could not read root block: %w", err)
	}

	var encodable flow.Block
	err = json.Unmarshal(bytes, &encodable)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal root block: %w", err)
	}
	return &encodable, nil
}

func ReadDKGData(dkgDataPath string) (*inmem.EncodableFullDKG, error) {
	bytes, err := io.ReadFile(dkgDataPath)
	if err != nil {
		return nil, fmt.Errorf("could not read dkg data: %w", err)
	}

	var encodable inmem.EncodableFullDKG
	err = json.Unmarshal(bytes, &encodable)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal dkg data: %w", err)
	}
	return &encodable, nil
}
