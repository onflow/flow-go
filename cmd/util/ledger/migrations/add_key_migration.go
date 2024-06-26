package migrations

import (
	"context"
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/crypto/hash"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// AddKeyMigration adds a new key to the core contracts accounts
type AddKeyMigration struct {
	log zerolog.Logger

	accountsToAddKeyTo map[common.Address]AddKeyMigrationAccountPublicKeyData
	chainID            flow.ChainID
}

var _ AccountBasedMigration = &AddKeyMigration{}

func NewAddKeyMigration(
	chainID flow.ChainID,
	key crypto.PublicKey,
) *AddKeyMigration {

	addresses := make(map[common.Address]AddKeyMigrationAccountPublicKeyData)
	sc := systemcontracts.SystemContractsForChain(chainID).All()

	for _, sc := range sc {
		addresses[common.Address(sc.Address)] = AddKeyMigrationAccountPublicKeyData{
			PublicKey: key,
			HashAlgo:  hash.SHA3_256,
		}
	}

	// add key to node accounts
	for _, nodeAddress := range mainnetNodeAddresses {

		address, err := common.HexToAddress(nodeAddress)
		if err != nil {
			// should never happen
			panic(fmt.Errorf("invalid node address %s: %w", nodeAddress, err))
		}

		addresses[address] = AddKeyMigrationAccountPublicKeyData{
			PublicKey: key,
			HashAlgo:  hash.SHA3_256,
		}
	}

	return &AddKeyMigration{
		accountsToAddKeyTo: addresses,
		chainID:            chainID,
	}
}

type AddKeyMigrationAccountPublicKeyData struct {
	PublicKey crypto.PublicKey
	HashAlgo  hash.HashingAlgorithm
}

func (m *AddKeyMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	_ int,
) error {
	m.log = log.With().Str("component", "AddKeyMigration").Logger()
	return nil
}

func (m *AddKeyMigration) Close() error {
	return nil
}

func (m *AddKeyMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	keyData, ok := m.accountsToAddKeyTo[address]
	if !ok {
		return nil
	}

	// Create all the runtime components we need for the migration
	migrationRuntime, err := NewInterpreterMigrationRuntime(
		accountRegisters,
		m.chainID,
		InterpreterMigrationRuntimeConfig{},
	)

	if err != nil {
		return err
	}

	account, err := migrationRuntime.Accounts.Get(flow.ConvertAddress(address))
	if err != nil {
		return fmt.Errorf("could not find account at address %s", address)
	}
	if len(account.Keys) == 0 {
		// this is unexpected,
		// all core contract accounts should have at least one key
		m.log.Warn().
			Str("address", address.String()).
			Msg("account has no keys")
	}

	key := flow.AccountPublicKey{
		PublicKey: keyData.PublicKey,
		SignAlgo:  keyData.PublicKey.Algorithm(),
		HashAlgo:  keyData.HashAlgo,
		Weight:    fvm.AccountKeyWeightThreshold,
	}

	err = migrationRuntime.Accounts.AppendPublicKey(flow.ConvertAddress(address), key)
	if err != nil {
		return err
	}

	// Finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes into the registers
	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	err = registers.ApplyChanges(
		accountRegisters,
		result.WriteSet,
		expectedAddresses,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to apply register changes: %w", err)
	}

	return nil

}

var mainnetNodeAddresses = []string{
	"731fff3c92213443", // "access-001.mainnet24.nodes.onflow.org:3569",
	"812c23d21adf3efd", // "access-002.mainnet24.nodes.onflow.org:3569",
	"8a06114bd65a40d4", // "access-003.mainnet24.nodes.onflow.org:3569",
	"7835cda55ea44a6a", // "access-004.mainnet24.nodes.onflow.org:3569",
	"05c726db6e2c35b5", // "access-005.mainnet24.nodes.onflow.org:3569",
	"00523f97886d672f", // "access-006.mainnet24.nodes.onflow.org:3569",
	"dfa97677e212fe54", // "access-007.mainnet24.nodes.onflow.org:3569",
	"decc3024db84c4f2", // "access-008.mainnet24.nodes.onflow.org:3569",
	"334b37c675df7ea8", // "access-009.mainnet24.nodes.onflow.org:3569",
	"f4e71586ea8c469e", // "access-010.mainnet24.nodes.onflow.org:3569",
	"8f930807301b124e", // "collection-001.mainnet24.nodes.onflow.org:3569",
	"7da0d4e9b8e518f0", // "collection-002.mainnet24.nodes.onflow.org:3569",
	"60de8343ed646cb8", // "collection-003.mainnet24.nodes.onflow.org:3569",
	"1644512674297fcc", // "collection-004.mainnet24.nodes.onflow.org:3569",
	"e4778dc8fcd77572", // "collection-005.mainnet24.nodes.onflow.org:3569",
	"ef5dbf5130520b5b", // "collection-006.mainnet24.nodes.onflow.org:3569",
	"1d6e63bfb8ac01e5", // "collection-007.mainnet24.nodes.onflow.org:3569",
	"eac8a61dd61359c1", // "collection-008.mainnet24.nodes.onflow.org:3569",
	"18fb7af35eed537f", // "collection-009.mainnet24.nodes.onflow.org:3569",
	"13d1486a92682d56", // "collection-010.mainnet24.nodes.onflow.org:3569",
	"e1e294841a9627e8", // "collection-011.mainnet24.nodes.onflow.org:3569",
	"e8022abba5379d6d", // "collection-012.mainnet24.nodes.onflow.org:3569",
	"1a31f6552dc997d3", // "collection-013.mainnet24.nodes.onflow.org:3569",
	"111bc4cce14ce9fa", // "collection-014.mainnet24.nodes.onflow.org:3569",
	"074fa1ff7848e39b", // "collection-015.mainnet24.nodes.onflow.org:3569",
	"f0e9645d16f7bbbf", // "collection-016.mainnet24.nodes.onflow.org:3569",
	"02dab8b39e09b101", // "collection-017.mainnet24.nodes.onflow.org:3569",
	"09f08a2a528ccf28", // "collection-018.mainnet24.nodes.onflow.org:3569",
	"fbc356c4da72c596", // "collection-019.mainnet24.nodes.onflow.org:3569",
	"0d00d5358d5ba714", // "collection-020.mainnet24.nodes.onflow.org:3569",
	"ff3309db05a5adaa", // "collection-021.mainnet24.nodes.onflow.org:3569",
	"f4193b42c920d383", // "collection-022.mainnet24.nodes.onflow.org:3569",
	"062ae7ac41ded93d", // "collection-023.mainnet24.nodes.onflow.org:3569",
	"f18c220e2f618119", // "collection-024.mainnet24.nodes.onflow.org:3569",
	"03bffee0a79f8ba7", // "collection-025.mainnet24.nodes.onflow.org:3569",
	"0895cc796b1af58e", // "collection-026.mainnet24.nodes.onflow.org:3569",
	"faa61097e3e4ff30", // "collection-027.mainnet24.nodes.onflow.org:3569",
	"f346aea85c4545b5", // "collection-028.mainnet24.nodes.onflow.org:3569",
	"01757246d4bb4f0b", // "collection-029.mainnet24.nodes.onflow.org:3569",
	"0a5f40df183e3122", // "collection-030.mainnet24.nodes.onflow.org:3569",
	"f86c9c3190c03b9c", // "collection-031.mainnet24.nodes.onflow.org:3569",
	"0fca5993fe7f63b8", // "collection-032.mainnet24.nodes.onflow.org:3569",
	"fdf9857d76816906", // "collection-033.mainnet24.nodes.onflow.org:3569",
	"f6d3b7e4ba04172f", // "collection-034.mainnet24.nodes.onflow.org:3569",
	"04e06b0a32fa1d91", // "collection-035.mainnet24.nodes.onflow.org:3569",
	"0db2761c119413f5", // "collection-036.mainnet24.nodes.onflow.org:3569",
	"ff81aaf2996a194b", // "collection-037.mainnet24.nodes.onflow.org:3569",
	"f4ab986b55ef6762", // "collection-038.mainnet24.nodes.onflow.org:3569",
	"06984485dd116ddc", // "collection-039.mainnet24.nodes.onflow.org:3569",
	"f13e8127b3ae35f8", // "collection-040.mainnet24.nodes.onflow.org:3569",
	"030d5dc93b503f46", // "collection-041.mainnet24.nodes.onflow.org:3569",
	"08276f50f7d5416f", // "collection-042.mainnet24.nodes.onflow.org:3569",
	"fa14b3be7f2b4bd1", // "collection-043.mainnet24.nodes.onflow.org:3569",
	"f3f40d81c08af154", // "collection-044.mainnet24.nodes.onflow.org:3569",
	"01c7d16f4874fbea", // "collection-045.mainnet24.nodes.onflow.org:3569",
	"0aede3f684f185c3", // "collection-046.mainnet24.nodes.onflow.org:3569",
	"f8de3f180c0f8f7d", // "collection-047.mainnet24.nodes.onflow.org:3569",
	"0f78faba62b0d759", // "collection-048.mainnet24.nodes.onflow.org:3569",
	"fd4b2654ea4edde7", // "collection-049.mainnet24.nodes.onflow.org:3569",
	"f66114cd26cba3ce", // "collection-050.mainnet24.nodes.onflow.org:3569",
	"0452c823ae35a970", // "collection-051.mainnet24.nodes.onflow.org:3569",
	"f2914bd2f91ccbf2", // "collection-052.mainnet24.nodes.onflow.org:3569",
	"00a2973c71e2c14c", // "collection-053.mainnet24.nodes.onflow.org:3569",
	"0b88a5a5bd67bf65", // "collection-054.mainnet24.nodes.onflow.org:3569",
	"ea7a05344adced20", // "collection-055.mainnet24.nodes.onflow.org:3569",
	"1849d9dac222e79e", // "collection-056.mainnet24.nodes.onflow.org:3569",
	"1363eb430ea799b7", // "collection-057.mainnet24.nodes.onflow.org:3569",
	"e15037ad86599309", // "collection-058.mainnet24.nodes.onflow.org:3569",
	"e8b0899239f8298c", // "collection-059.mainnet24.nodes.onflow.org:3569",
	"1a83557cb1062332", // "collection-060.mainnet24.nodes.onflow.org:3569",
	"11a967e57d835d1b", // "collection-061.mainnet24.nodes.onflow.org:3569",
	"e39abb0bf57d57a5", // "collection-062.mainnet24.nodes.onflow.org:3569",
	"143c7ea99bc20f81", // "collection-063.mainnet24.nodes.onflow.org:3569",
	"e60fa247133c053f", // "collection-064.mainnet24.nodes.onflow.org:3569",
	"ed2590dedfb97b16", // "collection-065.mainnet24.nodes.onflow.org:3569",
	"1f164c30574771a8", // "collection-066.mainnet24.nodes.onflow.org:3569",
	"5668f9ec131bd35f", // "collection-067.mainnet24.nodes.onflow.org:3569",
	"a45b25029be5d9e1", // "collection-068.mainnet24.nodes.onflow.org:3569",
	"af71179b5760a7c8", // "collection-069.mainnet24.nodes.onflow.org:3569",
	"5d42cb75df9ead76", // "collection-070.mainnet24.nodes.onflow.org:3569",
	"aae40ed7b121f552", // "collection-071.mainnet24.nodes.onflow.org:3569",
	"58d7d23939dfffec", // "collection-072.mainnet24.nodes.onflow.org:3569",
	"53fde0a0f55a81c5", // "collection-073.mainnet24.nodes.onflow.org:3569",
	"a1ce3c4e7da48b7b", // "collection-074.mainnet24.nodes.onflow.org:3569",
	"a82e8271c20531fe", // "collection-075.mainnet24.nodes.onflow.org:3569",
	"5a1d5e9f4afb3b40", // "collection-076.mainnet24.nodes.onflow.org:3569",
	"51376c06867e4569", // "collection-077.mainnet24.nodes.onflow.org:3569",
	"a304b0e80e804fd7", // "collection-078.mainnet24.nodes.onflow.org:3569",
	"54a2754a603f17f3", // "collection-079.mainnet24.nodes.onflow.org:3569",
	"a691a9a4e8c11d4d", // "collection-080.mainnet24.nodes.onflow.org:3569",
	"adbb9b3d24446364", // "collection-081.mainnet24.nodes.onflow.org:3569",
	"5f8847d3acba69da", // "collection-082.mainnet24.nodes.onflow.org:3569",
	"a94bc422fb930b58", // "collection-083.mainnet24.nodes.onflow.org:3569",
	"5b7818cc736d01e6", // "collection-084.mainnet24.nodes.onflow.org:3569",
	"50522a55bfe87fcf", // "collection-085.mainnet24.nodes.onflow.org:3569",
	"a261f6bb37167571", // "collection-086.mainnet24.nodes.onflow.org:3569",
	"55c7331959a92d55", // "collection-087.mainnet24.nodes.onflow.org:3569",
	"a7f4eff7d15727eb", // "collection-088.mainnet24.nodes.onflow.org:3569",
	"acdedd6e1dd259c2", // "collection-089.mainnet24.nodes.onflow.org:3569",
	"5eed0180952c537c", // "collection-090.mainnet24.nodes.onflow.org:3569",
	"570dbfbf2a8de9f9", // "collection-091.mainnet24.nodes.onflow.org:3569",
	"a53e6351a273e347", // "collection-092.mainnet24.nodes.onflow.org:3569",
	"ae1451c86ef69d6e", // "collection-093.mainnet24.nodes.onflow.org:3569",
	"5c278d26e60897d0", // "collection-094.mainnet24.nodes.onflow.org:3569",
	"4fe6f159994dcf2b", // "collection-095.mainnet24.nodes.onflow.org:3569",
	"bdd52db711b3c595", // "collection-096.mainnet24.nodes.onflow.org:3569",
	"b6ff1f2edd36bbbc", // "consensus-001.mainnet24.nodes.onflow.org:3569",
	"44ccc3c055c8b102", // "consensus-002.mainnet24.nodes.onflow.org:3569",
	"4d9eded676a6bf66", // "consensus-003.mainnet24.nodes.onflow.org:3569",
	"bfad0238fe58b5d8", // "consensus-004.mainnet24.nodes.onflow.org:3569",
	"b48730a132ddcbf1", // "consensus-005.mainnet24.nodes.onflow.org:3569",
	"46b4ec4fba23c14f", // "consensus-006.mainnet24.nodes.onflow.org:3569",
	"b11229edd49c996b", // "consensus-007.mainnet24.nodes.onflow.org:3569",
	"ac6c7e47811ded23", // "consensus-008.mainnet24.nodes.onflow.org:3569",
	"5e5fa2a909e3e79d", // "consensus-009.mainnet24.nodes.onflow.org:3569",
	"57bf1c96b6425d18", // "consensus-010.mainnet24.nodes.onflow.org:3569",
	"a58cc0783ebc57a6", // "consensus-011.mainnet24.nodes.onflow.org:3569",
	"aea6f2e1f239298f", // "consensus-012.mainnet24.nodes.onflow.org:3569",
	"5c952e0f7ac72331", // "consensus-013.mainnet24.nodes.onflow.org:3569",
	"ab33ebad14787b15", // "consensus-014.mainnet24.nodes.onflow.org:3569",
	"590037439c8671ab", // "consensus-015.mainnet24.nodes.onflow.org:3569",
	"522a05da50030f82", // "consensus-016.mainnet24.nodes.onflow.org:3569",
	"447e60e9c90705e3", // "consensus-017.mainnet24.nodes.onflow.org:3569",
	"b2bde3189e2e6761", // "consensus-018.mainnet24.nodes.onflow.org:3569",
	"408e3ff616d06ddf", // "consensus-019.mainnet24.nodes.onflow.org:3569",
	"4ba40d6fda5513f6", // "consensus-020.mainnet24.nodes.onflow.org:3569",
	"b997d18152ab1948", // "consensus-021.mainnet24.nodes.onflow.org:3569",
	"4e3114233c14416c", // "consensus-022.mainnet24.nodes.onflow.org:3569",
	"bc02c8cdb4ea4bd2", // "consensus-023.mainnet24.nodes.onflow.org:3569",
	"b728fa54786f35fb", // "consensus-024.mainnet24.nodes.onflow.org:3569",
	"451b26baf0913f45", // "consensus-025.mainnet24.nodes.onflow.org:3569",
	"4cfb98854f3085c0", // "consensus-026.mainnet24.nodes.onflow.org:3569",
	"bec8446bc7ce8f7e", // "consensus-027.mainnet24.nodes.onflow.org:3569",
	"b5e276f20b4bf157", // "consensus-028.mainnet24.nodes.onflow.org:3569",
	"47d1aa1c83b5fbe9", // "consensus-029.mainnet24.nodes.onflow.org:3569",
	"b0776fbeed0aa3cd", // "consensus-030.mainnet24.nodes.onflow.org:3569",
	"4244b35065f4a973", // "consensus-031.mainnet24.nodes.onflow.org:3569",
	"496e81c9a971d75a", // "consensus-032.mainnet24.nodes.onflow.org:3569",
	"bb5d5d27218fdde4", // "consensus-033.mainnet24.nodes.onflow.org:3569",
	"cdc78f42b8c2ce90", // "consensus-034.mainnet24.nodes.onflow.org:3569",
	"3ff453ac303cc42e", // "consensus-035.mainnet24.nodes.onflow.org:3569",
	"34de6135fcb9ba07", // "consensus-036.mainnet24.nodes.onflow.org:3569",
	"c6edbddb7447b0b9", // "consensus-037.mainnet24.nodes.onflow.org:3569",
	"314b78791af8e89d", // "consensus-038.mainnet24.nodes.onflow.org:3569",
	"c378a4979206e223", // "consensus-039.mainnet24.nodes.onflow.org:3569",
	"c852960e5e839c0a", // "consensus-040.mainnet24.nodes.onflow.org:3569",
	"01fb1c432a2fed81", // "execution-001.mainnet24.nodes.onflow.org:3569",
	"17af7970b32be7e0", // "execution-002.mainnet24.nodes.onflow.org:3569",
	"e009bcd2dd94bfc4", // "execution-003.mainnet24.nodes.onflow.org:3569",
	"123a603c556ab57a", // "execution-004.mainnet24.nodes.onflow.org:3569",
	"88cc9deda57e8478", // "verification-001.mainnet24.nodes.onflow.org:3569",
	"191052a599efcb53", // "verification-002.mainnet24.nodes.onflow.org:3569",
	"eb238e4b1111c1ed", // "verification-003.mainnet24.nodes.onflow.org:3569",
	"e271935d327fcf89", // "verification-004.mainnet24.nodes.onflow.org:3569",
	"10424fb3ba81c537", // "verification-005.mainnet24.nodes.onflow.org:3569",
	"1b687d2a7604bb1e", // "verification-006.mainnet24.nodes.onflow.org:3569",
	"e95ba1c4fefab1a0", // "verification-007.mainnet24.nodes.onflow.org:3569",
	"1efd64669045e984", // "verification-008.mainnet24.nodes.onflow.org:3569",
	"ecceb88818bbe33a", // "verification-009.mainnet24.nodes.onflow.org:3569",
	"e7e48a11d43e9d13", // "verification-010.mainnet24.nodes.onflow.org:3569",
	"15d756ff5cc097ad", // "verification-011.mainnet24.nodes.onflow.org:3569",
	"1c37e8c0e3612d28", // "verification-012.mainnet24.nodes.onflow.org:3569",
	"ee04342e6b9f2796", // "verification-013.mainnet24.nodes.onflow.org:3569",
	"e52e06b7a71a59bf", // "verification-014.mainnet24.nodes.onflow.org:3569",
	"171dda592fe45301", // "verification-015.mainnet24.nodes.onflow.org:3569",
	"e0bb1ffb415b0b25", // "verification-016.mainnet24.nodes.onflow.org:3569",
	"1288c315c9a5019b", // "verification-017.mainnet24.nodes.onflow.org:3569",
	"19a2f18c05207fb2", // "verification-018.mainnet24.nodes.onflow.org:3569",
	"eb912d628dde750c", // "verification-019.mainnet24.nodes.onflow.org:3569",
	"1d52ae93daf7178e", // "verification-020.mainnet24.nodes.onflow.org:3569",
	"ef61727d52091d30", // "verification-021.mainnet24.nodes.onflow.org:3569",
	"e44b40e49e8c6319", // "verification-022.mainnet24.nodes.onflow.org:3569",
	"16789c0a167269a7", // "verification-023.mainnet24.nodes.onflow.org:3569",
	"e1de59a878cd3183", // "verification-024.mainnet24.nodes.onflow.org:3569",
	"13ed8546f0333b3d", // "verification-025.mainnet24.nodes.onflow.org:3569",
	"18c7b7df3cb64514", // "verification-026.mainnet24.nodes.onflow.org:3569",
	"eaf46b31b4484faa", // "verification-027.mainnet24.nodes.onflow.org:3569",
	"e314d50e0be9f52f", // "verification-028.mainnet24.nodes.onflow.org:3569",
	"112709e08317ff91", // "verification-029.mainnet24.nodes.onflow.org:3569",
	"1a0d3b794f9281b8", // "verification-030.mainnet24.nodes.onflow.org:3569",
	"e83ee797c76c8b06", // "verification-031.mainnet24.nodes.onflow.org:3569",
	"1f982235a9d3d322", // "verification-032.mainnet24.nodes.onflow.org:3569",
	"edabfedb212dd99c", // "verification-033.mainnet24.nodes.onflow.org:3569",
	"e681cc42eda8a7b5", // "verification-034.mainnet24.nodes.onflow.org:3569",
	"14b210ac6556ad0b", // "verification-035.mainnet24.nodes.onflow.org:3569",
	"6228c2c9fc1bbe7f", // "verification-036.mainnet24.nodes.onflow.org:3569",
	"901b1e2774e5b4c1", // "verification-037.mainnet24.nodes.onflow.org:3569",
	"9b312cbeb860cae8", // "verification-038.mainnet24.nodes.onflow.org:3569",
	"6902f050309ec056", // "verification-039.mainnet24.nodes.onflow.org:3569",
	"9ea435f25e219872", // "verification-040.mainnet24.nodes.onflow.org:3569",
	"6c97e91cd6df92cc", // "verification-041.mainnet24.nodes.onflow.org:3569",
	"71e9beb6835ee684", // "verification-042.mainnet24.nodes.onflow.org:3569",
	"780900893cff5c01", // "verification-043.mainnet24.nodes.onflow.org:3569",
	"8a3adc67b40156bf", // "verification-044.mainnet24.nodes.onflow.org:3569",
	"65775723697e2849", // "verification-045.mainnet24.nodes.onflow.org:3569",
	"97448bcde18022f7", // "verification-046.mainnet24.nodes.onflow.org:3569",
	"60e24e6f8f3f7ad3", // "verification-047.mainnet24.nodes.onflow.org:3569",
	"92d1928107c1706d", // "verification-048.mainnet24.nodes.onflow.org:3569",
	"99fba018cb440e44", // "verification-049.mainnet24.nodes.onflow.org:3569",
	"6bc87cf643ba04fa", // "verification-050.mainnet24.nodes.onflow.org:3569",
}
