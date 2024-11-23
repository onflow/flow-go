package state

import (
	"io"
	"os"
	"path/filepath"

	"github.com/onflow/atree"
	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/model/flow"
)

const (
	ExportedAccountsFileName = "accounts.bin"
	ExportedCodesFileName    = "codes.bin"
	ExportedSlotsFileName    = "slots.bin"
)

type Exporter struct {
	ledger   atree.Ledger
	root     flow.Address
	baseView *BaseView
}

// NewExporter constructs a new Exporter
func NewExporter(ledger atree.Ledger, root flow.Address) (*Exporter, error) {
	bv, err := NewBaseView(ledger, root)
	if err != nil {
		return nil, err
	}
	return &Exporter{
		ledger:   ledger,
		root:     root,
		baseView: bv,
	}, nil
}

func (e *Exporter) Export(path string) error {
	af, err := os.Create(filepath.Join(path, ExportedAccountsFileName))
	if err != nil {
		return err
	}
	defer af.Close()

	addrWithStorage, err := e.exportAccounts(af)
	if err != nil {
		return err
	}

	cf, err := os.Create(filepath.Join(path, ExportedCodesFileName))
	if err != nil {
		return err
	}
	defer cf.Close()

	err = e.exportCodes(cf)
	if err != nil {
		return err
	}

	sf, err := os.Create(filepath.Join(path, ExportedSlotsFileName))
	if err != nil {
		return err
	}
	defer cf.Close()

	err = e.exportSlots(addrWithStorage, sf)
	if err != nil {
		return err
	}
	return nil
}

// exports accounts and returns a list of addresses with non-empty storage
func (e *Exporter) exportAccounts(writer io.Writer) ([]gethCommon.Address, error) {
	itr, err := e.baseView.AccountIterator()
	if err != nil {
		return nil, err
	}
	// make a list of accounts with storage
	addrWithSlots := make([]gethCommon.Address, 0)
	for {
		// TODO: we can optimize by returning the encoded value
		acc, err := itr.Next()
		if err != nil {
			return nil, err
		}
		if acc == nil {
			break
		}
		if acc.HasStoredValues() {
			addrWithSlots = append(addrWithSlots, acc.Address)
		}
		encoded, err := acc.Encode()
		if err != nil {
			return nil, err
		}
		// write every account on a new line
		_, err = writer.Write(append(encoded, byte('\n')))
		if err != nil {
			return nil, err
		}
	}
	return addrWithSlots, nil
}

// exportCodes exports codes
func (e *Exporter) exportCodes(writer io.Writer) error {
	itr, err := e.baseView.CodeIterator()
	if err != nil {
		return err
	}
	for {
		cic, err := itr.Next()
		if err != nil {
			return err
		}
		if cic == nil {
			break
		}
		encoded, err := cic.Encode()
		if err != nil {
			return err
		}
		// write every codes on a new line
		_, err = writer.Write(append(encoded, byte('\n')))
		if err != nil {
			return err
		}
	}
	return nil
}

// exportSlots exports slots (key value pairs stored under accounts)
func (e *Exporter) exportSlots(addresses []gethCommon.Address, writer io.Writer) error {
	for _, addr := range addresses {
		itr, err := e.baseView.AccountStorageIterator(addr)
		if err != nil {
			return err
		}
		for {
			slot, err := itr.Next()
			if err != nil {
				return err
			}
			if slot == nil {
				break
			}
			encoded, err := slot.Encode()
			if err != nil {
				return err
			}
			// write every codes on a new line
			_, err = writer.Write(append(encoded, byte('\n')))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
