package generate_authorization_fixes

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/onflow/cadence/runtime/common"
)

// AddressPublicPath is a public path in an account.
type AddressPublicPath struct {
	Address    common.Address
	Identifier string
}

type LinkInfo struct {
	BorrowType        common.TypeID
	AccessibleMembers []string
}

// PublicLinkReport is a mapping from public account paths to link info.
type PublicLinkReport map[AddressPublicPath]LinkInfo

// ReadPublicLinkReport reads a link report from the given reader.
// The report is expected to be a JSON array of objects with the following structure:
//
//	[
//		{"address":"0x1","identifier":"foo","linkType":"&Foo","accessibleMembers":["foo"]}
//	]
func ReadPublicLinkReport(
	reader io.Reader,
	filter map[common.Address]struct{},
) (PublicLinkReport, error) {

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

		if filter != nil {
			if _, ok := filter[address]; !ok {
				continue
			}
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
