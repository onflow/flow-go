package generate_authorization_fixes

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/onflow/cadence/runtime/common"
)

// AccountCapabilityID is a capability ID in an account.
type AccountCapabilityID struct {
	Address      common.Address
	CapabilityID uint64
}

// MigratedPublicLinkSet is a set of capability controller IDs which were migrated from public links.
type MigratedPublicLinkSet map[AccountCapabilityID]struct{}

// ReadMigratedPublicLinkSet reads a link migration report from the given reader,
// and returns a set of all capability controller IDs which were migrated from public links.
//
// The report is expected to be a JSON array of objects with the following structure:
//
//	[
//		{"kind":"link-migration-success","account_address":"0x1","path":"/public/foo","capability_id":1},
//	]
func ReadMigratedPublicLinkSet(
	reader io.Reader,
	filter map[common.Address]struct{},
) (MigratedPublicLinkSet, error) {

	set := MigratedPublicLinkSet{}

	dec := json.NewDecoder(reader)

	token, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	if token != json.Delim('[') {
		return nil, fmt.Errorf("expected start of array, got %s", token)
	}

	for dec.More() {
		var entry struct {
			Kind         string `json:"kind"`
			Address      string `json:"account_address"`
			Path         string `json:"path"`
			CapabilityID uint64 `json:"capability_id"`
		}
		err := dec.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}

		if entry.Kind != "link-migration-success" {
			continue
		}

		if !strings.HasPrefix(entry.Path, "/public/") {
			continue
		}

		address, err := common.HexToAddress(entry.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to parse address: %w", err)
		}

		if filter != nil {
			if _, ok := filter[address]; !ok {
				continue
			}
		}

		accountCapabilityID := AccountCapabilityID{
			Address:      address,
			CapabilityID: entry.CapabilityID,
		}
		set[accountCapabilityID] = struct{}{}
	}

	token, err = dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	if token != json.Delim(']') {
		return nil, fmt.Errorf("expected end of array, got %s", token)
	}

	return set, nil
}
