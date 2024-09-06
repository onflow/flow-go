package generate_entitlement_fixes

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

// PublicLinkMigrationReport is a mapping from account capability controller IDs to public path identifier.
type PublicLinkMigrationReport map[AccountCapabilityID]string

// ReadPublicLinkMigrationReport reads a link migration report from the given reader,
// and extracts the public paths that were migrated.
//
// The report is expected to be a JSON array of objects with the following structure:
//
//	[
//		{"kind":"link-migration-success","account_address":"0x1","path":"/public/foo","capability_id":1},
//	]
func ReadPublicLinkMigrationReport(
	reader io.Reader,
	filter map[common.Address]struct{},
) (PublicLinkMigrationReport, error) {

	mapping := PublicLinkMigrationReport{}

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

		identifier, ok := strings.CutPrefix(entry.Path, "/public/")
		if !ok {
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

		key := AccountCapabilityID{
			Address:      address,
			CapabilityID: entry.CapabilityID,
		}
		mapping[key] = identifier
	}

	token, err = dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	if token != json.Delim(']') {
		return nil, fmt.Errorf("expected end of array, got %s", token)
	}

	return mapping, nil
}
