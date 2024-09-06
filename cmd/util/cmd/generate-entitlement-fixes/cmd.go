package generate_entitlement_fixes

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/onflow/cadence/runtime/common"
)

type AccountCapabilityControllerID struct {
	Address      common.Address
	CapabilityID uint64
}

type LinkInfo struct {
	BorrowType        common.TypeID
	AccessibleMembers []string
}

type AddressPublicPath struct {
	Address    common.Address
	Identifier string
}

// PublicLinkReport is a mapping from public account paths to link info.
type PublicLinkReport map[AddressPublicPath]LinkInfo

// ReadPublicLinkReport reads a link report from the given reader.
// The report is expected to be a JSON array of objects with the following structure:
//
//	[
//		{"address":"0x1","identifier":"foo","linkType":"&Foo","accessibleMembers":["foo"]}
//	]
func ReadPublicLinkReport(reader io.Reader) (PublicLinkReport, error) {
	report := PublicLinkReport{}

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
			Address           string   `json:"address"`
			Identifier        string   `json:"identifier"`
			LinkTypeID        string   `json:"linkType"`
			AccessibleMembers []string `json:"accessibleMembers"`
		}
		err := dec.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}

		address, err := common.HexToAddress(entry.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to parse address: %w", err)
		}

		key := AddressPublicPath{
			Address:    address,
			Identifier: entry.Identifier,
		}
		report[key] = LinkInfo{
			BorrowType:        common.TypeID(entry.LinkTypeID),
			AccessibleMembers: entry.AccessibleMembers,
		}
	}

	token, err = dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	if token != json.Delim(']') {
		return nil, fmt.Errorf("expected end of array, got %s", token)
	}

	return report, nil
}

// PublicLinkMigrationReport is a mapping from account capability controller IDs to public path identifier.
type PublicLinkMigrationReport map[AccountCapabilityControllerID]string

// ReadPublicLinkMigrationReport reads a link migration report from the given reader,
// and extracts the public paths that were migrated.
//
// The report is expected to be a JSON array of objects with the following structure:
//
//	[
//		{"kind":"link-migration-success","account_address":"0x1","path":"/public/foo","capability_id":1},
//	]
func ReadPublicLinkMigrationReport(reader io.Reader) (PublicLinkMigrationReport, error) {
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

		key := AccountCapabilityControllerID{
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
