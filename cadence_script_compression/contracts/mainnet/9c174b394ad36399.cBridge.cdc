import Pb from 0x9c174b394ad36399
// only need to support signer verify for now
pub contract cBridge {
    // path for admin resource
    pub let AdminPath: StoragePath
    // unique chainid required by cbridge system
    pub let chainID: UInt64
    access(contract) let domainPrefix: [UInt8]
    // save map from pubkey to power, only visible in this contract to extra safety
    access(contract) var signers: {String: UInt256}
    pub var totalPower: UInt256
    // block height when signers are last updated by sigs
    // flow block.timestamp is UFix64 so hard to use
    pub var signerUpdateBlock: UInt64

    // at first we require pubkey to be also present in SignerSig, so verify can just do map lookup in signers
    // but this make other components in the system to be aware of pubkeys which is hard to do
    // so without pubkey, we just loop over all signers and call verify, it's less efficient but won't be an issue
    // as there won't be many signers. we keep struct for future flexibility but pubkey field is NOT needed.
    pub struct SignerSig {
        pub let sig: [UInt8]
        init(sig: [UInt8]) {
            self.sig = sig
        }
    }

    // Admin is a special authorization resource that 
    // allows the owner to perform important functions to modify the contract state
    pub resource BridgeAdmin {
        pub fun resetSigners(signers: {String: UInt256}) {
            cBridge.signers = signers
            cBridge.totalPower = 0
            for power in signers.values {
                cBridge.totalPower = cBridge.totalPower + power
            }
        }
        pub fun createBridgeAdmin(): @BridgeAdmin {
            return <-create BridgeAdmin()
        }
    }

    // getter for signers/power as self.signers is not accessible outside contract
    pub fun getSigners(): {String: UInt256} {
        return self.signers
    }

    // return true if sigs have more than 2/3 total power
    pub fun verify(data:[UInt8], sigs: [SignerSig]): Bool {
        var signedPower: UInt256 = 0
        var quorum: UInt256 = (self.totalPower*2)/3 + 1
        // go over all known signers, for each signer, try to find which sig matches
        for pubkey in self.signers.keys {
            let pk = PublicKey(
                    publicKey: pubkey.decodeHex(),
                    signatureAlgorithm: SignatureAlgorithm.ECDSA_secp256k1
            )
            // for this pk, try to find the matched sig
            // flow doc says `for index, element in array` but it's only in master https://github.com/onflow/cadence/issues/1230
            var idx = 0
            for ss in sigs {
                if pk.verify(
                    signature: ss.sig,
                    signedData: data,
                    domainSeparationTag: "FLOW-V0.0-user",
                    hashAlgorithm: HashAlgorithm.SHA3_256
                ) {
                    signedPower = signedPower + self.signers[pubkey]!
                    if signedPower >= quorum {
                        return true
                    }
                    // delete ss from sigs
                    sigs.remove(at: idx)
                    // now we break the sig loop to check next signer as we only allow one sig per signer
                    break
                }
                idx = idx + 1
            }
        }
        return false
    }

    init(chID:UInt64) {
      self.chainID = chID
      // domainPrefix is chainID big endianbytes followed by "A.xxxxxx.cBridge".utf8, xxxx is this contract account
      self.domainPrefix = chID.toBigEndianBytes().concat(self.getType().identifier.utf8)
  
      self.signers = {}
      self.totalPower = 0

      self.signerUpdateBlock = 0

      self.AdminPath = /storage/cBridgeAdmin
      self.account.save<@BridgeAdmin>(<- create BridgeAdmin(), to: self.AdminPath)
    }

    // below are struct and func needed to update cBridge.signers by cosigned msg
    // blockheight is like nonce to avoid replay attack
    /* proto definition
    message PbSignerPowerMap {
        uint64 blockheight = 1; // block when this pb was proposed by sgn, must < currentblock
        repeated SignerPower = 2;
    }
    message SignerPower {
        bytes pubkey = 1; // raw bytes of pubkey, 64 bytes
        bytes power = 2; // big.Int.Bytes
    }
    */
    pub struct PbSignerPowerMap {
        pub var blockheight: UInt64
        pub let signerMap: {String: UInt256}
        pub var totalPower: UInt256
        // raw is serialized PbSignerPowerMap
        init(_ raw: [UInt8]) {
            self.blockheight = 0
            self.signerMap = {}
            self.totalPower = 0
            // now decode pbraw and assign
            let buf = Pb.Buffer(raw)
            while buf.hasMore() {
              let tagType = buf.decKey()
              switch Int(tagType.tag) {
                case 1:
                  assert(tagType.wt==Pb.WireType.Varint, message: "wrong wire type")
                  self.blockheight = Pb.toUInt64(buf.decBytes())
                case 2:
                  assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
                  // now decode SignerPower msg
                  let tag1 = buf.decKey()
                  assert(tag1.tag==1, message: "tag not 1: ".concat(tag1.tag.toString()))
                  assert(tag1.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
                  let pbhex = Pb.toString(buf.decBytes())
                  let tag2 = buf.decKey()
                  assert(tag2.tag==1, message: "tag not 2: ".concat(tag2.tag.toString()))
                  assert(tag2.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
                  let power = Pb.toUInt256(buf.decBytes())
                  // add to map
                  self.signerMap[pbhex] = power
                  self.totalPower = self.totalPower + power
                default:
                  assert(false, message: "unsupported tag")
              }
            }
        }
    }
    // new signers can be updated if cosigned by existing signers w/ 2/3 power
    // pbmsg is serialized PbSignerPowerMap
    pub fun updateSigners(pbmsg:[UInt8], sigs: [SignerSig]) {
      let domain = self.domainPrefix.concat(".UpdateSigners".utf8)
      assert(self.verify(data: domain.concat(pbmsg), sigs: sigs), message: "verify sigs failed")
      let newSigners = PbSignerPowerMap(pbmsg)
      assert(newSigners.totalPower > 0, message: "total power must > 0")
      assert(newSigners.blockheight > self.signerUpdateBlock, message: "new signer timestamp must be larger")
      assert(newSigners.blockheight <= getCurrentBlock().height, message: "new signer timestamp must <= current")
      // update signerUpdateBlock and signers
      self.signerUpdateBlock = getCurrentBlock().height
      self.totalPower = newSigners.totalPower
      self.signers = newSigners.signerMap
    }
}
 