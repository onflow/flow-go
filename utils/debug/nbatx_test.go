package debug_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestDebugger_RunTransaction(t *testing.T) {

	grpcAddress := "35.192.34.155:9000"
	chain := flow.Mainnet.Chain()
	debugger := debug.NewRemoteDebugger(grpcAddress, chain, zerolog.New(os.Stdout).With().Logger())

	const scriptStr = `
	import TopShot from 0x0b2a3299cc857e29
	import TopShotShardedCollection from 0xef4d8b44dd7f7ef6
	
	transaction {
		prepare(acct: AuthAccount) {
			let recipient = getAccount(0xe1f2a091f7bb5245)
			let receiverRef = recipient.getCapability(/public/MomentCollection)!
				.borrow<&{TopShot.MomentCollectionPublic}>()
				?? panic("Could not borrow reference to receiver''s collection")
	
			let momentIDs = [UInt64(26086254),7141395,10302796,3402886,19902846,19328136,11358489,18852801,13005829,23376246,14450288,6377202,22718356,26338444,28073054,10814414,18829007,30347201,8797476,7439682,22158846,19110691,5046011,16093930,28526112,6426383,15753231,26623452,10180633,20855330,16080228,15682339,7453222,28063055,32723106,23806387,13417218,13146535,23489061,29571274,19539697,26559934,24743603,23692406,24263723,27080191,9812627,2972722,28553430,15862798,12898559,20649153,24669293,25017654,25095981,13201653,21753372,21466053,28603696,9724671,15696731,26221845,22234814,24194198,32087521,8208118,22445436,28310836,24118820,20365132,27088496,6190135,24190850,25547426,34766096,27516458,1333926,33118140,14075650,31264240,28470500,34026427,14519251,27956683,26842978,27464159,22102176,33445702,12332913,18696088,21986652,28977299,25257366,23782963,12829688,26288946,15626906,20467187,25561320,13115261,12653836,26503272,11600928,8649764,23994903,25363642,6521476,26604249,15995256,13417167,3113040,6595605,13036090,23123790,13736698,28325249,27488841,31030948,27377408,32721718,4520370,27818377,19713607,13100129,29496936]
	
			let shardedCollection = acct.borrow<&TopShotShardedCollection.ShardedCollection>
				(from: /storage/TopShotShardedCollection)!
	
			let normalCollection = acct.borrow<&TopShot.Collection>(from: /storage/MomentCollection)!
	
			for momentID in momentIDs {
				if shardedCollection.borrowMoment(id: momentID) != nil {
					receiverRef.deposit(token: <-shardedCollection.withdraw(withdrawID: momentID))
				} else {
					receiverRef.deposit(token: <-normalCollection.withdraw(withdrawID: momentID))
				}
			}
		}
	}
	`

	script := []byte(scriptStr)
	acc := flow.HexToAddress("0xfa57101aa0d55954")
	txBody := flow.NewTransactionBody().
		SetGasLimit(9999).
		SetScript([]byte(script)).
		SetPayer(chain.ServiceAddress()).
		SetProposalKey(chain.ServiceAddress(), 0, 0)
	txBody.Authorizers = []flow.Address{acc}

	// Run with blockID (use the file cache)
	blockId, err := flow.HexStringToIdentifier("1497bd595f6616bdea56ffbcc49ccaff94df961da261ad8aef69b75813fbb189")
	require.NoError(t, err)

	testCacheFile := "nba_tx.cache"
	txErr, err := debugger.RunTransactionAtBlockID(txBody, blockId, testCacheFile)
	require.NoError(t, txErr)
	require.NoError(t, err)
}
