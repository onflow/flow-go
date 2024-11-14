package utils_test

import (
	"bufio"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/offchain/utils"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

const resume_height = 6559268

func TestValues(t *testing.T) {
	values, err := deserialize("./values.gob")
	require.NoError(t, err)
	fmt.Println(len(values))

	rootAddr := evm.StorageAccountAddress(flow.Testnet)
	for fk := range values {
		owner, _, err := decodeFullKey(fk)
		require.NoError(t, err)
		require.Equal(t, owner, rootAddr[:])
	}

	allocators, err := deserializeAllocator("./allocators.gob")
	require.NoError(t, err)
	fmt.Println(len(allocators))
	for k, v := range allocators {
		fmt.Println(k, v)
	}
}

func decodeFullKey(encoded string) ([]byte, []byte, error) {
	// Split the encoded string at the first occurrence of "~"
	parts := strings.SplitN(encoded, "~", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid encoded key: no delimiter found")
	}

	// Convert the split parts back to byte slices
	owner := []byte(parts[0])
	key := []byte(parts[1])
	return owner, key, nil
}

func TestTestnetFrom(t *testing.T) {
	values, err := deserialize("./values.gob")
	require.NoError(t, err)
	allocators, err := deserializeAllocator("./allocators.gob")
	require.NoError(t, err)
	store := GetSimpleValueStorePopulated(values, allocators)
	ReplayingFromSratchFromHeight(t, flow.Testnet, store,
		fmt.Sprintf("/Users/leozhang/Downloads/devnet51_evm_events_%v.jsonl", resume_height), resume_height)
}

func TestTestnetTo(t *testing.T) {
	ReplayingFromSratchToHeight(t, flow.Testnet, GetSimpleValueStore(),
		"/Users/leozhang/Downloads/devnet51_evm_events.jsonl", resume_height-1)
}

func fixedHashes() []gethCommon.Hash {
	hashes := []string{"fa857cb5d4b774e975d149a91dc47687ab6400301bba7fab1a70e82bd57ab33b", "57c87eeb449e976020fb60b3366b867ffc9d88ec5c0f10171af4c7c771462130", "1af58b777b8054a15f3e0c60ec1c0501bd7626003a4fabb2017e16f1f4f9b0aa", "155eb38e56a75c59863434446071a29df399e0b79a0f7627f3c0def08c0dee4a", "fc541a457aaacc00c4bbf2ddd296c212c7c7436a1b15fdf40971436f4679060b", "22461d010d68d2b67a7a3373782af7f75eb240a845c4b1fa1c399c48f7d3eaad", "e62881132d705937c2a0f88cd0e94f595e922e752f5a3225ecbb4e4f91f242e5", "61084954ebe8d12d9ac71a9ce32f2f72c5ab819ab3382215e0122b98ef98bf6d", "b65786186ff332a66cf502565101ed3fdf0a005d8ea847829a909cafc948cdb7", "6ec4b77f75ce5bd028a22f88049d856dbf83b34480f24eb13ed567de839e06f9", "c1db0ccc2f546863cda1e14da73d951e4fa4c788427f13500a1a7557709de271", "b8a6f83f59913bece208fbc481bdf8a0ac332433f8cb01a3c5c1b7ae377f2700", "9a8c588bb81d8c622b8c6d9073233c176440da4dce49433b56398c30239cfe8d", "a84205d415780ed3c0566f9f4578efeb6ec4ca51f8a93cc7f89a00ccce8dcb39", "c5d6591d91eef2ca446351e95dc4134438360c1b7389d975d636cbacba435280", "7be74dcff396c8abf98c6727659575a5b157c9ec98c6f1c9504732054f09aaf5", "a7dcad11df6d5778824decb3624953440a2e8f01036083c10adb36b4465ee14a", "ac6e904295a3d736e7f22ecb5698c1fd8964e3f0afc07ee2487e63ee606b9bbf", "d7c2cff7f8a08373b8aed134fe1fa80899ddaaa8dd7722fca9b2954228b25803", "580cf925b0d2ec1617e17f0be43402381d537e789bd5a08c3a681dcdcae2d731", "c71cde092dddca890f9f44567a651434a801119dfca6fa6a8ad6daa26ce4d6a2", "b010526b4edd19af408eae841184d97f1ad6e8955c4a6ac8240e32f75a26e5f9", "9278a4d8204e7b937c41c71b9f03c97c49203d4cc6e4e6d429be80ff1d11bf02", "d57366198709ee6be52ea72cb54cfb6282ddd6708e487839f74b93c06c9a994a", "1d17a3f34d23425ad6fa3b1f57cb1276d988c3064c727995cd6966af22323830", "660a0a66a46fae20c0a4f2b1a5f11c246ce39bc1338f641ea304cf2dc9bd0940", "e4562f14b6464d2ee4e92764b6126fab3b37b12c8b0ccb0cbc539a0f1d54318f", "3ed39df06d960213a978379790386ec1c6df288a524c9bc11dbc869d1133e86d", "f09abfcf424b6bcb7a54fc613828e5ff756b619c957c51457d833efbbfd9c601", "58b6fe973b269639c2a6dc768e1f1f328c3c1d098b6ded3511b1f8e3393f8344", "398fd65258285061025e5b53043496832acca2a6b61906046605df18767a9da3", "b933d1d819cdbeff8e3acf9cba0fe7b3e6db3bb582da027a0f1e432219bd6033", "99baada49d56352f2e221cf62116c70485a83c1174bcd50cf5ba62b35d1661a9", "19a47884389d1f995a37c7e2b19525d44a27a32a5df2c0b9c2954fe458655baf", "3820ea36958821d31b8f2eee80fc17e72dcf361f052c0399931ce979e9a10293", "3a3655c7bb4fb1814002b468d63f72c0626d4c7df4ceac28a68c970a3686712a", "bc181caec490ade2d715e7d0c82cb9ad3fd685dc962d8ffca00861d88f5366b4", "da92ccf74d37b40738c41222cee137c149889966c54d62d91472d2ea81be37f2", "e51d0d81a40598e0d6281d2bbc56a1d8c5aa3c8233f2bd9be2316ad6a24a2dc3", "6cb2e1aea92658471cc40ec0a4bfd64d8e76bc0b9bb5707306fe89d93158e7c4", "e4bb2e67f5ff721ecfac0df301bf3db9704d47a9d33c2f952be17dc23a113c45", "7d29bf4f9796573cf5274900ec667bced39cb0377409d281a2dbceaf99ec8fd9", "45b32bbc856daf25ad81206623f8a7fb53f0afbb488f72ffef4d8f0a9431e62b", "b5aca33f4af1f65d9e9e35035597b58896d99abd5b7954593ffc70c86a90c94d", "7a21bc1136bd1b288fb5be1fd43b39cdfeae9b424e3da274e241dbc1ac780d72", "95bd53bea9d44609b8b24ff5c30feb08c91d92f239632f8093fbb8f37a704112", "61551f4fb10bd3b97870af25c6c18d8582d6badef8e87e3c5297befd1331003a", "ba43a4bd43dcdf44ce163b58d35df3def39f2a2ed29cfcf76f3d7571827b8bc1", "329c277c2f0555d33e294377bf906c404a163ee653d0894661714a25b1d3c8fb", "6e143a6cf96b0b8eb695bd77b1e28f2a61f4dac8a47b3cf2b69d6737d8441242", "991bc0911f5914677f4ba476717a53b0b889b91cf178ae66c0625167f7ac0801", "541fb4e3a4fc928a017bdce01393ea8113b2236dafbb3809973f7b8352442d32", "9c9181ad53d6506666187974b6b9e3a9c0bee8d085d10cc79f50bfb4248ca129", "1cb89bb5668ac284574be9118a78d3fa5d674c84579c75d5596a47d2acce29f6", "116b1c4d1a8fef4cd852a8841b689fff4f1df3a0f5bbeb545942150f4b806646", "b54f3b2b235b816bda74453e228378fcf9b79a293534aac71dbfeb6b0ee1ecad", "9acb23972960f0b4c5d3c6b061a2a1c4af4f7a6d4a0cdd8ec7134ae7bd59f95d", "17f3d6c720bc5efd5ee8226d353d1b347828e621400a2a282a190f5b7bbdd0f0", "1838dc6001bb37cff89aa8675ec0ae8efdfd35c5dc8a793538c31d08df4b8232", "ad362ed3de8ac036d4a89d31282f26e10cb50fa900c6ad76f7ab06cb7155d234", "2bd6a5464607a39d0bcdd07e15d4752d1a52b644bf9a81d8d7e5f9cff0af30af", "44124bbba59755b9004d53c3e721820c40c1cc163b7639b4c1a03ce6955e292b", "f19520a13533371cea4cc20daeef421c31c0a88d4604e58b56ebef82288cdaaf", "c1796053a6e8847cf3d8a545670dd953d1273dd3d9a6e4df6e59e33950cc2890", "49aeb76ef737a04fe91c3a61dc8c7b87adad5978d8951f8d033ddeae6fa2b720", "bed2427fb70a9a9a576528569ccfd8fc86ab0ecd4ca7a932d5a8f39316f887a0", "a8da98fa12885b4165f7635906d9bc240c2eaa66079bf18f496dbecb68c7c49e", "cd7e523f67b5ab520d1c8972f78db9a8d283c66ccf000aa31cda8216fe2e508b", "1e29c627ce7b6402eb5115c59a48d561f4420c44748d7de2ed185142beab4a29", "5ddf101e94858f06934c6019eaa22b93d88eca16592720e9dcd982894ac27060", "c408705873fb0ab3fc4f5811e69ee20b0a1600f52bb4663e29362f4391601ebe", "ddf70a2c37e60622148124c22f8f0e96b4eba0af4d5b8b18015d574f33923a7e", "d6e1f406e0d96c486c1bcbb09768ff0e5577f18c97cdf2c3e86dda54b4007448", "656b861ba19271a6591c7468af61a9d29e331eccc9e526a3d25517d29bd69809", "24372783456ac149b4fd0dc41ee16d55500a3c433fc3b1bd3c1c45c8a93c89c5", "2bbbb4392ab7f1fd8a160a80163b69b5f8db16fdf97c2d8ee9e29df1d9ebd9fe", "cc9fd404792808740bdee891c8e93e3d41bfe56c2438396d1ca8a692dd5fb990", "38080ff661e3142133b82633be87af6db2d33f386d05f8439672a1984aa88d13", "22b7125bf763c17087306776783ab6d1c50084e8a7435b015207f99295aa1af9", "570c31b148e5f909873e8d2253401a64eace826993948cd2f3f4d03a798c6c54", "f0cb29da50bff805a3a1736dbe33ea139893534d0e25a98f354aa5f279adbc97", "cd6b07cee12ae00058b20a6d31173c934933e6339a00885554ccefde008b12e3", "323fa87c41960355883ada3b85bbc13303d8202761ea70d015841060c7f7fde7", "01c7c87db4a01af781695e2984e68b72f04a0f7859749bcfdcbee73466bf0990", "a79003be6397a1fac1d183ebd14d72f69cfd9ab310cd8f9cc9c3d835b05d7556", "50dcfbe053447768b56f6c3159cc6d37aa5791d87abfad32b2952e36de8a20c7", "21647bb0680b8b09b357a54518a50d6c4163d78889f26ef48bc93cfe43acb16d", "96dfe03bc8aa7dd74ef98b4cb7cad866c851b8fd145f4b5bdb54c7b799e58adf", "87037ff5508a2a31c62cbef1feb19f3ec22f44ade292e0a036e8a7d8ef3d13bd", "6e7336d4e63a744ae45cfd320ca237ba4b194d930bcbfbfde2d172616df367b8", "780126f3f77af11cac4a71371812160e436d50f09ee01eb312d6839b7dd4e3a3", "9373a2bdc426bc5bf3242c7f3ecc83a19f2cfc0772ecdeb846e423fc8ec40b5e", "0339e7901bccba1e3c8e05956536823b2b0e7189c66f5796b7602b63a8fd1ff9", "b213bb94b274991d4288a6405954059e99b4c4b891a74a1abcd83ea295331b18", "d0a7195ec0cd987709b4dc6416e0ed6fc9939054ecbf502da8c4c6a09836ed9c", "7b9c334b3aeb75a795f9d6c7c0ee01ab219f31860880eb3480921dcd2a057d2b", "9c4e722d126467603530d242fe91a19fa90ebd3a461ee38f36ef5eefa07e996c", "4306ac8ccd2ce6a880350f95c7d59635371ba3d78bb13353c5b7ff06f7c6fc40", "4b9360e2d86f20850d2c6ff222ed16c6a4252c00afad8d488c30c162b3a10da7", "927f20b9dcfbb80f4a6b5d6067a586835bdcb5f3e921ed87bec67fb5160181d1", "e620bc51fbeb8011f57324b0a7ae6f45c46050cd624887f0a50879880632fdaa", "ee7b749b81e86d46fa3e93b9aba29285bae38a91f175dbf7c619d05fcf91e857", "573d5039fa570ceb3fa136be73c432b49a19af00a7f109325b78160f7dc13db1", "9ab1936825e830d4eab7a945701528579f78a8d1702a76a774e7456ddd3a254e", "2b3538a6fed897c0143f51b82f7e9e1929cb698e7de8d88aa8b1d23cabd58fa9", "21e2f8ae0522da985262ccf8422d98d75068ccd448d15c4bfec9f793713c7644", "c02a276e24fbb64f5b35d4b6555d1d873095e076868cea8dcfdad9e606612f9b", "7756adb6b470c5126693a4de57c1d5b38afab4f7ffc4f982374e8466051bcfca", "f82cbe9343e63fa4bf486f8e4113f91abef7c994e6f7068b500942fede79f095", "782f9df4e3f669149a575922a7318d523b1ab8a5911a2b1c2850839d5762cf03", "89ef33e05604e28f762b3cdf2f20d876adcb104a87c2636c5facb61ec47d020c", "59e374462a0c7e32df5e087d4d250936ef54aa19ca824ebaa63b66406180719d", "11fc2b68e458f12e93398a453c5efac599691bd89d40c35e003dc594d87bf51d", "5f793edc159efab968da834bd44187fff951cec822ca1b8982b1f36d966956be", "da0d474d5e0ec5d0966e1986a5de3f085e0f491da67cdb43d52fdc9848b14314", "8d4eec56231819d18f3fb3ec6e6881b269c0ccb881eedecb5916d2b4ef82c6cf", "137e7ea7c47a724f8a4494a3e73e74f146282382935d64d25385dd720f537e98", "1a2a9c7707443c848897141a4f659fbd0b7fefa47365f2af43183777dcb4a8ef", "7747a6f738959e6d75f16fe6d0782b455258b9c93d0380a230722cd6ae11e0bb", "314e30caef6c7c09b2a85056610949febb6abbbf7702c5d6706cef658123d782", "9ab42848b175c62790b5aa4f256899bb609d05723d364b8d349160afadfd9f95", "853b07dda09eb155dcebbac23e2fa5d76c5f619f3cabfa5e25fd82706485bd25", "a2b0053632aafe21d4dff287c03c362cae2a1d3267cd87d82a7ba9a3795129c9", "7918541145cb2c5918b8fa20a31298a7bc9b8f43aebb69f046f78d070a7f22ef", "0827e91cf9ec4dbb95966d68cdeb90dc8399457f47922d1e53eb2972c87756ef", "6121dac0131fc1fe0f7652d6c2195141c0e6a9b7e5cb555647ec3bb2f90b912d", "134fae4eec772042a832efc19e2f3e449db962f3573c070f2920591c306967b1", "b9a716636f3d1dd47e61aa1216f55317230cf734e06c9f740552f2bbd6e8210a", "d5caa5c0bb57e75c78de5f6f132e19776b777dd205d37ff6c2179412caa32c40", "e11c15139b71e7078a664d430e115c631ac8cdd89a8f4b35e4bbbeb9ec85dc17", "cbff909b284e4b1858adff2a0cee75032a2b2411d805604dfe820e40e855d6b5", "5b4ce1b89dde6b8b5cbec1b454306b7f53a9dadcdbe5df429ea5a33635d989d3", "c06a55411e962e0bf9cc11c14e854be084906b374cc181868c29ebcab0b66775", "ad16c4f73055baa8c0c6f69e294019ea90e3e97ee90923c4478156e15180d19c", "76866d7b50747a469e9891c529b7a58a4b9082d113b7acbe2b46f6049a8d36c7", "df96c9eba4763a1c3a8a0d2eb14e57847ce679adeda80b04cb86ef4f40cf290a", "6421d33aed4529b00db819051abed4ae78f28778feab921177c24378d48b427b", "cb76cbf3c146f5890eef6a8e78349b9291b75d2ca3b947b027f52dab0acbcdd4", "cb9a9e1606d5d6cc59bce096733be7e6902d8c8de19d22cc0f5435ad4e719015", "d3b9005c6b93a657d8edd2312d4d59b8807ea7c509079dfd1e4a8cef3d6852ba", "ebc705fd3ee20a69c5e99b1bf063acff8c926eec9358a36294b8df0fdcd31eb5", "c99e64329e066cc19b2e9962bfa2eb474bb7f9bd1c797421878209c16ca85d80", "55a081aab8afb0cfe83873b812c4495a762bdfc866d74c038d64f73d26944db3", "d830b389b67743e2a2cec5d64af37ce1b991b2781bf2a3fb1e8283bc78e98495", "558d06ff221f4d6e5265465ef2928828a80b498f95d7b1853c4a93d842931ccd", "e967f7ac0177971566b44535eed88a5ffcd0b2ec09de03edbf817f8e110eaf5b", "404df2a8bcf278cae68d9a43b86ff9c2781461ccd227c20aa5e0c5b1db2c0cb1", "f8f5160a6d1e91a3cae676b1e8f8563da2e1cb92869df51c190f0d91f62c81b2", "30a23be3cb0e3feab447217745d537e6c5299f3a95172c234bb84de54169b694", "7c5e66106c5e7cb9e68cd6bec431acdb4b0c9394f2c000a60f0ec558b1667750", "be103be330df170331a747138325af15173704afb808abfd6fc5742c677de241", "d711f0d3914c1bee36324e055ade9058750f2b3d0206f516382702de8eda3757", "519658c8746832821044b074a40661ea1497ca50426888303d8eec43ae8b9d6a", "87cd56d2f6ff774a0c75b029c2a888df7b41319380336f3e4663fe5417229687", "2efe240e7018fd0443262223d286c04120199063f4ef194bdef9af0ab34fa4a8", "8c9a69c950bea4e4beecc286124bf44e2cc78614f767580d59dc22cf94bd23f6", "e7641851ddf32f8fa1937528a2c88a2ef512d45f0a7296c232df6584471ad7ba", "a5beb770e26085eb45a6c5e15acb5844fdda167261e92b20c87dc72c1e0d0a1d", "f54988150d2ba3327251b7a4672ec9bff6fe93f06a7a9f19030f17e693281f11", "6cbfa48ae32ef9b3798f0afe4b86798497a758735dc3ac3e0aa6b42710476f58", "35130215ec7db0e57d5964dacb9aa2ea858e70fc864edd08cf062334823a3ce8", "a935e9ddece310c12baa815a0077e151b300a293f88651d7715ea33151d4016e", "167e10bb4d35aa27a4916de2f846ff5d323a0090c9d37b9c35ca455272ab07be", "a85a1222927f535ca37587d38ab4db2bc940bfc0c6d703003119329d05469a75", "826ab7e279754c009dcd86421f3bdaaa3325bdfff8352788c9f8cfbdddfcfafe", "8336015f3f6ca5d69d5af6dfd521a3e3c024c08121bd42de3a25e5bffb417d42", "194125cbe3f428afbf59da1dd144062ad288011e10beca10ca534f935ea7290d", "3bb36e7a0165d3b6f51b628c18e6b4d9e355b05c5be7a616c881dc395c623c66", "c092f7add11cee0facec22c78badba46fb8688538df1443b7356ceea83bae10d", "31552a2bb308a5778e815fee39b007ce5a633d2e7ba27f08eee2bec6f8d387b7", "373533933e0aae2d2dcffb59b09c49fa64506606aa0359eddf00326ee7bbcc7e", "0f580299cabe89b2dc9809735d14fdabf60cc1b65824bb5f6b5cf283b68210ee", "138c02ce7b36a4d7e82f942a3291bbb357b2e8845b579189ce4c35e01e6b859f", "9dc184037f271c4043b1a6d01d9fbed5d2f156fb561ec2612e5b1cd6aa486083", "cb2ae942cd73059bfe666d9ef78cee5a557cda842c9503df0f7d6b00be815cc5", "f941433597eaa923318023f040798918f743db7bf6d33bc6a13bc8c2e8d3e711", "02a1f2c523e2705b1ab122a06c08bd64080ef76d09d517c56c4e64a3f6626021", "ac3dda90e10c66d26ebb6911924713785f48e8e3d2150aa06ae90db456e1c9a0", "61a39a58e915f953d1ea5c0483f3f45b33ed6f097d76ea6d03d7cf81616f33bd", "b3ff677201fa7543da2f635753305a128c4076409268f1ee53ee824989193e90", "3a2cf44822616731ce40cde80365738e4a4d9af161de3cc2bb3e4f4d3ced8009", "b1a3f23c441a6afece152c4b2e1f1da6fc952f997bc8711a6122e26afafeb5b1", "87123bd9968d64fead15b346ad4ef3b0918aebc596fb7ce8c016c09085985bbd", "eae98597fc685154c882a62073157e1538e37270573de17e7f9bd1af724e1164", "e6dc4cfe6c4b77ebc2a915a49157447a65f85c275ba6c888fddbfae95a2d1c2b", "45ffffa2166eff3624a6b83e5d953669e3639188556330a58656d51ac9008f15", "2b5658b7d00f6d34890e71cf1d57b520e934f6b4087cca5c50604a7c8190488d", "d9b516ec359cccafc8cd2c5721bed137cb0d4b7bb21ba4772baed786a9f059a6", "5005e282fff3675ff3ac18906d5cf9df5b992d0bd95fc9cd3258f386f1c5b5ea", "2dea763455c4ae2c662bd9db6529b85cfd397744cb3da1a639925b0fa2b048b4", "497399dc295066a487984ab67cbfec9bf3d65184bc424a7b96268f2c03e6557f", "8f87e5ab712b41e1bc6f74fd74bb8e96323f62f62bedb35ed578992ddbbd5f47", "a5504fbce2afcd7277b0bd94581050195607d5c6701cff8d8e25f05a2d50d81c", "205b534ee10a3633f87c8ab36590d114f516985470ef5851077ac5c95aa83f16", "0d2093c088c08840643f542a44d9e8c389694f03dc9c62a264445de5758e73c3", "b32c1de573b72b62ce6b77d628f758acbfe89ecaa17d3c4c94cad8dff45dd0c9", "6d75d744de2e5dd7ebd3fa47b22ca0d99d4255ee36b5e767567479e0134e0697", "d3228e2e8e5de7178f2afc4b6f86b13287469b55410a164397bc602a0e3bd2db", "5d0a5e9e280f90c7d1f69b69ff3b5bfb94bce299dde8799520fe92912afd2cff", "aafabddd3fe15559af9138aa113c2473fed25a41ee52877a05dc2f9b24416827", "00a9160b3ae08d4066e53992f3cc004b3f6bf3d840613d6e847fb16323ddb270", "1473078fe8d18a5e3f791064c1083783fdc19517a3f2af47777d8778bb2b2f89", "aa2b720f1b7fd016086641fa0c3a6f8133c5f7eb3e9a65cd01ad0b51e7c35719", "ecdd45371e9a284e97416f414d665afa0aec864277a03c333e785e4d6ba6d439", "66a7301e8f3d54360b15fc64610398888301a3caeb685dc71e0ec0fdd175937f", "bd156dc25f23d82eaef927957d4c8c883ec0c80de4c58310313764ccc701d281", "3e8aa53535920d5886779d30687c2350800e9c712c5c2414db463b9c99f3052e", "308a237dad23fa158e7590ce7c75e788ec3ae6be8f6972a867f2eb94f6417c96", "4b12d020e1df286f672fe5d2eac74d95f817d0bbb8bee15a7913ebd9c3a8014a", "303c6f66eaff75bf2145e3bcc343245bcbedb2df46af1fb1e8382473fd2ab402", "d21e974892bd9209a0e2333b22acb55ec2a4abc015755379640cb81d4ba38d82", "40bdb0c10ce735f5e6abf18bf46dd8ef5625ea828fbfc6e380b70809d7cf76dd", "c0b4d28f557f71bcc41eb3573e2afb6da0c127639972bbcb8f4962cff0896f7a", "d2e36f3773f4c313fafb160ac753f1a11b53783920d45552b693f7a37b80bbe2", "3fd160ad0045137801256a22fed09f5f31aacf31f1681fbf6d70bc03972d2253", "2c0c05796774bdbb27c0a6ec5559817b4cd48feee80dff2c540257f86733e397", "5f17ad7ebf06c9ee5f7c86716e2392fd65b773eb6c94f47ac1ea1e12afbacfd0", "0dc16b207a0a9a722cd0b6ce18419eeb2c7809a9f90f3ebca7cc084d6714469d", "8f576a107b37c1309055282825effed4d57dd7e96fd69595ad300c26f77b07a5", "b433f6a339e84a5dbd8e6638a4547dd029b642d1007199948678d7574350b64e", "738768e552067738d3ba97fabe8ea93c0a6ba3b64cc24fab0e9b0c2ce4842982", "533020acb857afd489d4766280665cc484d184ed8eeaacd031e8a5e70b5c4a88", "1d84007a810cb751a5f7207b36cffb1a7f50c1553cbcb0c922c7cb1ada8bb409", "a0b398eb392174cfa24948edcf03c50553a7367c7f6ed50970456484ea09680b", "f156f642f5fd502eb9d0fff911981506c32e6c40b12362e6b3082dffb7fc6550", "2338e90aafd734d44bd50aad3f4d0f4255e2d2505546925e810798626c79f4f4", "c141f87ed878c297468d5be367ce8df0c7d90be4b6be070059eb9345f8250b62", "57106030bb89bd435844ef9baf318c9696af10784a4cf09359bff4b22a4d74eb", "2419aa33614ded3307173c53d6f614b6567d6f50fdc9a99fd32a299efc3de982", "07e60b9438f0b0fc97151c34b781b2a6370cb4d6c48ecbbfe0016a24ebe7bf31", "c09518a1b22c36e3d599af9f956090609fff015a794680a12f730364c721aae4", "65ddb5cf2927237525c5b3d3613eb346660cba60d0478ea917b6f0aa4907d7d9", "1c8935ede01448904447520b90c742615062e404f3525fe5bd667e06f7341c13", "fcb9e121eb526413ec8c827a3dda5e619a85ffbcb7508f0525ac22a121a100f5", "6d0a6422309f64d722ba79f621a4fe3db0ecf16b40366313b146a97d95667307", "4ad7e9d2a199b2eb3cc1cf7bb35e7b03a0ca18bd7382ed29a18b97ed01cd63a0", "69e377941f0263ce3c585789ae6106782d1f15db0b1942a9627b2bd6fe83e13d", "bb51ed5948d59b0dcb2f5cb5f8a27d3f70b8c71660b0d6d4ab658b6a7ca2356c", "6695e79e0e07fde8c05da60736ba373d55271d5a7c6da2a2c7d30e957a46e7e7", "48bd888c98b158b5c82b148f091a91bb1881b9a1931227f0a5269649a8eebaff", "771382cfa5138ccd32fdddad18e3eb8f1a06eee10704248d1e4d49f32872afe6", "176bb2e118aeb292912fa1903470621ae385e819a50c580301b33165666f3c7d", "15596f8c5f8fb397e5214e6f5eaf286a813b6e5d8bebae2bad1d550511f92840", "bdacbb2d763783f1ac51fd2477276543f79db13a434697a2aedd8523a1427e1f", "ca0e3b746890e8d626840d445989bb0e703f3e4c792aaa49a6b8952ea7696063", "1319af4c3801a463f0e1b7a9cfe2cfbb79e769fb0daed1a2868ade7665765ea6", "172f67582c5270cf0ef8264ef64bc5e17a53aac87693eff1860dfe56aea4209e", "0462589f719e853654d1ca00038dfc806ae7acb9bb5a3f9e6d458f3d4206f532", "f7480a6f46b553517f41238cbd5a6069eab164fd1512e1685f9bddf5c1afa59c", "a5cfdbe5c0b38b0904b5fe6afc2ce583dce1dbc7b4cd88224cbd88efa30b0291", "6f07b548ced6405ef78693332d516d041780f85f0771cfbaba8bbb86a6cdfb7d", "de0184abac150e780e26f1e7de09da64dfee433e8c9a9efe8d93a673350016b8", "8e7cee539c6315ad939a9495e40e7e70e2d07f6b2920cdbcc689457cd9e11997", "0088ccc025bf814e8098607bfbd17448024495a62610700b6000ec448afc1ca3", "d3a0503fdb8802e979871dca7d3c10a928cedf1978e44f42ecb72b96ada13dc3", "add0b405d079dd0c682a1e5026ef1a5b989b0bdf044d2db28249b4d51a74c5dc"}
	// Initialize a slice of [32]byte to store the converted hashes
	var gethHashes []gethCommon.Hash

	// Convert each string to a [32]byte
	for _, hash := range hashes {
		// Decode hex string to bytes
		// Append to the slice
		gethHashes = append(gethHashes, gethCommon.HexToHash(hash))
	}

	return gethHashes
}

func ReplayingFromSratchFromHeight(
	t *testing.T,
	chainID flow.ChainID,
	storage *TestValueStore,
	filePath string,
	startExecutingFromHeight uint64,
) {
	rootAddr := evm.StorageAccountAddress(chainID)
	bp, err := blocks.NewBasicProvider(chainID, storage, rootAddr)

	hashes := fixedHashes()

	ReplayingBlocksFromScratch(t, storage, filePath,
		func(blockEventPayload *events.BlockEventPayload, txEvents []events.TransactionEventPayload) error {
			if blockEventPayload.Height < startExecutingFromHeight {
				return nil
			}

			fmt.Println("height: ", blockEventPayload.Height)

			idx := blockEventPayload.Height % 256

			blockEventPayload.Hash = hashes[idx]

			err = bp.OnBlockReceived(blockEventPayload)
			require.NoError(t, err)

			sp := NewTestStorageProvider(storage, blockEventPayload.Height)
			cr := sync.NewReplayer(chainID, rootAddr, sp, bp, zerolog.Logger{}, nil, true)
			res, err := cr.ReplayBlock(txEvents, blockEventPayload)
			require.NoError(t, err)
			// commit all changes
			for k, v := range res.StorageRegisterUpdates() {
				err = storage.SetValue([]byte(k.Owner), []byte(k.Key), v)
				require.NoError(t, err)
			}

			err = bp.OnBlockExecuted(blockEventPayload.Height, res)
			require.NoError(t, err)

			return nil
		})
}

func ReplayingFromSratchToHeight(
	t *testing.T,
	chainID flow.ChainID,
	storage *TestValueStore,
	filePath string,
	stopAfterExecuted uint64,
) {
	rootAddr := evm.StorageAccountAddress(chainID)
	// setup the rootAddress account
	as := environment.NewAccountStatus()
	err := storage.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
	require.NoError(t, err)

	bp, err := blocks.NewBasicProvider(chainID, storage, rootAddr)
	require.NoError(t, err)

	ReplayingBlocksFromScratch(t, storage, filePath,
		func(blockEventPayload *events.BlockEventPayload, txEvents []events.TransactionEventPayload) error {
			fmt.Println("height: ", blockEventPayload.Height)
			err = bp.OnBlockReceived(blockEventPayload)
			require.NoError(t, err)

			sp := NewTestStorageProvider(storage, blockEventPayload.Height)
			cr := sync.NewReplayer(chainID, rootAddr, sp, bp, zerolog.Logger{}, nil, true)
			res, err := cr.ReplayBlock(txEvents, blockEventPayload)
			require.NoError(t, err, fmt.Sprintf("failed to replay block at height %v: %v", blockEventPayload.Height, err))
			// commit all changes
			for k, v := range res.StorageRegisterUpdates() {
				err = storage.SetValue([]byte(k.Owner), []byte(k.Key), v)
				require.NoError(t, err)
			}

			err = bp.OnBlockExecuted(blockEventPayload.Height, res)
			require.NoError(t, err)

			if blockEventPayload.Height == stopAfterExecuted {
				values, allocators := storage.Dump()
				require.NoError(t, serialize("./values.gob", values))
				require.NoError(t, serializeAllocator("./allocators.gob", allocators))
				fmt.Println("finished writing for height ", blockEventPayload.Height)

				newValues, err := deserialize("./values.gob")
				require.NoError(t, err)
				newAllocators, err := deserializeAllocator("./allocators.gob")
				require.NoError(t, err)

				checkMapConsistent(t, values, newValues)
				checkAllocatorConsistent(t, allocators, newAllocators)
				fmt.Println("finished checking for height ", blockEventPayload.Height)
				return fmt.Errorf("exeucted height reached target height: %v", stopAfterExecuted)
			}
			return nil
		})
}

func ReplayingFromSratch(
	t *testing.T,
	chainID flow.ChainID,
	storage *TestValueStore,
	filePath string,
) {
	rootAddr := evm.StorageAccountAddress(chainID)

	// setup the rootAddress account
	as := environment.NewAccountStatus()
	err := storage.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
	require.NoError(t, err)

	bp, err := blocks.NewBasicProvider(chainID, storage, rootAddr)
	require.NoError(t, err)

	ReplayingBlocksFromScratch(t, storage, filePath,
		func(blockEventPayload *events.BlockEventPayload, txEvents []events.TransactionEventPayload) error {
			err = bp.OnBlockReceived(blockEventPayload)
			require.NoError(t, err)

			sp := NewTestStorageProvider(storage, blockEventPayload.Height)
			cr := sync.NewReplayer(chainID, rootAddr, sp, bp, zerolog.Logger{}, nil, true)
			res, err := cr.ReplayBlock(txEvents, blockEventPayload)
			require.NoError(t, err)
			// commit all changes
			for k, v := range res.StorageRegisterUpdates() {
				err = storage.SetValue([]byte(k.Owner), []byte(k.Key), v)
				require.NoError(t, err)
			}

			err = bp.OnBlockExecuted(blockEventPayload.Height, res)
			require.NoError(t, err)

			return nil
		})
}

func ReplayingBlocksFromScratch(
	t *testing.T,
	storage *TestValueStore,
	filePath string,
	handler func(*events.BlockEventPayload, []events.TransactionEventPayload) error,
) {
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	txEvents := make([]events.TransactionEventPayload, 0)

	for scanner.Scan() {
		data := scanner.Bytes()
		var e utils.Event
		err := json.Unmarshal(data, &e)
		require.NoError(t, err)
		if strings.Contains(e.EventType, "BlockExecuted") {
			temp, err := hex.DecodeString(e.EventPayload)
			require.NoError(t, err)
			ev, err := ccf.Decode(nil, temp)
			require.NoError(t, err)
			blockEventPayload, err := events.DecodeBlockEventPayload(ev.(cadence.Event))
			require.NoError(t, err)

			require.NoError(t, handler(blockEventPayload, txEvents), fmt.Sprintf("fail to handle block at height %d",
				blockEventPayload.Height))

			txEvents = make([]events.TransactionEventPayload, 0)
			continue
		}

		temp, err := hex.DecodeString(e.EventPayload)
		require.NoError(t, err)
		ev, err := ccf.Decode(nil, temp)
		require.NoError(t, err)
		txEv, err := events.DecodeTransactionEventPayload(ev.(cadence.Event))
		require.NoError(t, err)
		txEvents = append(txEvents, *txEv)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveEventsUpToHeight(t *testing.T) {
	RemoveEventsUpToHeight(t, "/Users/leozhang/Downloads/devnet51_evm_events.jsonl", resume_height-1,
		fmt.Sprintf("/Users/leozhang/Downloads/devnet51_evm_events_%v.jsonl", resume_height))
}

func RemoveEventsUpToHeight(t *testing.T, filePath string, height uint64, newFilePath string) {
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	newFile, err := os.Create(newFilePath)
	require.NoError(t, err)
	defer newFile.Close()

	writing := false

	for scanner.Scan() {
		data := scanner.Bytes()
		var e utils.Event
		err := json.Unmarshal(data, &e)
		require.NoError(t, err)
		if strings.Contains(e.EventType, "BlockExecuted") {
			temp, err := hex.DecodeString(e.EventPayload)
			require.NoError(t, err)
			ev, err := ccf.Decode(nil, temp)
			require.NoError(t, err)
			blockEventPayload, err := events.DecodeBlockEventPayload(ev.(cadence.Event))
			require.NoError(t, err)

			if !writing {
				writing = blockEventPayload.Height == height
				continue
			}
		}

		if writing {
			_, err = newFile.Write(append(data, '\n'))
			require.NoError(t, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

// Serialize function: saves map data to a file
func serialize(filename string, data map[string][]byte) error {
	// Create a file to save data
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use gob to encode data
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

// Deserialize function: reads map data from a file
func deserialize(filename string) (map[string][]byte, error) {
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Prepare the map to store decoded data
	var data map[string][]byte

	// Use gob to decode data
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Serialize function: saves map data to a file
func serializeAllocator(filename string, data map[string]uint64) error {
	// Create a file to save data
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use gob to encode data
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

// Deserialize function: reads map data from a file
func deserializeAllocator(filename string) (map[string]uint64, error) {
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Prepare the map to store decoded data
	var data map[string]uint64

	// Use gob to decode data
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func checkMapConsistent(t *testing.T, oldMap map[string][]byte, newMap map[string][]byte) {
	require.Equal(t, len(oldMap), len(newMap))
	for k, v := range oldMap {
		require.Equal(t, v, newMap[k])
	}
}

func checkAllocatorConsistent(t *testing.T, oldMap map[string]uint64, newMap map[string]uint64) {
	require.Equal(t, len(oldMap), len(newMap))
	for k, v := range oldMap {
		require.Equal(t, v, newMap[k])
	}
}
