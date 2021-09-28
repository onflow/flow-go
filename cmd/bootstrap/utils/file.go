package utils

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
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

func ReadRootBlockData(rootBlockDataPath string) (*inmem.EncodableRootBlockData, error) {
	bytes, err := io.ReadFile(rootBlockDataPath)
	if err != nil {
		return nil, fmt.Errorf("could not read root block data: %w", err)
	}

	var encodable inmem.EncodableRootBlockData
	err = json.Unmarshal(bytes, &encodable)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal root block data: %w", err)
	}
	return &encodable, nil
}

func ReadDKGPubData(dkgDataPath string) (*inmem.EncodableDKG, error) {
	bytes, err := io.ReadFile(dkgDataPath)
	if err != nil {
		return nil, fmt.Errorf("could not read dkg pub data: %w", err)
	}

	var encodable inmem.EncodableDKG
	err = json.Unmarshal(bytes, &encodable)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal dkg pub data: %w", err)
	}
	return &encodable, nil
}

func ReadDKGParticipant(dkgParticipantPath string) (*dkg.DKGParticipantPriv, error) {
	bytes, err := io.ReadFile(dkgParticipantPath)
	if err != nil {
		return nil, fmt.Errorf("could not read dkg participant: %w", err)
	}

	var encodable dkg.DKGParticipantPriv
	err = json.Unmarshal(bytes, &encodable)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal dkg participant: %w", err)
	}
	return &encodable, nil
}
