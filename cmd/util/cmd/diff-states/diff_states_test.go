package diff_states

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

func newPayload(owner flow.Address, key string, value []byte) *ledger.Payload {
	registerID := flow.NewRegisterID(owner, key)
	ledgerKey := convert.RegisterIDToLedgerKey(registerID)
	return ledger.NewPayload(ledgerKey, value)
}

func TestDiffStates(t *testing.T) {

	unittest.RunWithTempDir(t, func(datadir string) {

		file1 := filepath.Join(datadir, "first.payloads")
		file2 := filepath.Join(datadir, "second.payloads")

		payloads1 := []*ledger.Payload{
			newPayload(flow.Address{1}, "a", []byte{2}),
			newPayload(flow.Address{1}, "b", []byte{3}),
			newPayload(flow.Address{2}, "c", []byte{4}),
		}
		payloads2 := []*ledger.Payload{
			newPayload(flow.Address{1}, "a", []byte{2}),
			newPayload(flow.Address{1}, "b", []byte{5}),
			newPayload(flow.Address{3}, "d", []byte{6}),
			newPayload(flow.Address{4}, "e", []byte{7}),
		}

		numOfPayloadWritten, err := util.CreatePayloadFile(zerolog.Nop(), file1, payloads1, nil, false)
		require.NoError(t, err)
		require.Equal(t, len(payloads1), numOfPayloadWritten)

		numOfPayloadWritten, err = util.CreatePayloadFile(zerolog.Nop(), file2, payloads2, nil, false)
		require.NoError(t, err)
		require.Equal(t, len(payloads2), numOfPayloadWritten)

		Cmd.SetArgs([]string{
			"--payloads-1", file1,
			"--payloads-2", file2,
			"--chain", string(flow.Emulator),
			"--output-directory", datadir,
			"--raw",
		})

		err = Cmd.Execute()
		require.NoError(t, err)

		var reportPath string
		err = filepath.Walk(
			datadir,
			func(path string, info fs.FileInfo, err error) error {
				if path != datadir && info.IsDir() {
					return filepath.SkipDir
				}
				if strings.HasPrefix(filepath.Base(path), ReporterName) {
					reportPath = path
					return filepath.SkipAll
				}
				return err
			},
		)
		require.NoError(t, err)
		require.NotEmpty(t, reportPath)

		report, err := io.ReadFile(reportPath)
		require.NoError(t, err)

		assert.JSONEq(
			t,
			`
              [
                {"kind":"raw-diff", "owner":"0100000000000000", "key":"62", "value1":"03", "value2":"05"},
                {"kind":"account-missing", "owner":"0200000000000000", "state":2},
                {"kind":"account-missing", "owner":"0300000000000000", "state":1},
                {"kind":"account-missing", "owner":"0400000000000000", "state":1}
              ]
            `,
			string(report),
		)

	})
}
