import Crypto
import FungibleToken from 0xf233dcee88fe0abe

pub contract OnChainMultiSig {
    
    
    // ------- Events ------- 
    pub event NewPayloadAdded(resourceId: UInt64, txIndex: UInt64);
    pub event NewPayloadSigAdded(resourceId: UInt64, txIndex: UInt64);

    
    // ------- Interfaces ------- 
    pub resource interface PublicSigner {
        pub fun addNewPayload(payload: @PayloadDetails, publicKey: String, sig: [UInt8]);
        pub fun addPayloadSignature (txIndex: UInt64, publicKey: String, sig: [UInt8]);
        pub fun executeTx(txIndex: UInt64): @AnyResource?;
        pub fun UUID(): UInt64;
        pub fun getTxIndex(): UInt64;
        pub fun getSignerKeys(): [String];
        pub fun getSignerKeyAttr(publicKey: String): PubKeyAttr?;
    }
    
    
    pub resource interface KeyManager {
        pub fun addKeys( multiSigPubKeys: [String], multiSigKeyWeights: [UFix64], multiSigAlgos: [UInt8]);
        pub fun removeKeys( multiSigPubKeys: [String]);
    }
    

    pub resource interface SignatureManager {
        pub fun getSignerKeys(): [String];
        pub fun getSignerKeyAttr(publicKey: String): PubKeyAttr?;
        pub fun addNewPayload (resourceId: UInt64, payload: @PayloadDetails, publicKey: String, sig: [UInt8]);
        pub fun addPayloadSignature (resourceId: UInt64, txIndex: UInt64, publicKey: String, sig: [UInt8]);
        pub fun readyForExecution(txIndex: UInt64): @PayloadDetails?;
        pub fun configureKeys (pks: [String], kws: [UFix64], sa: [UInt8]);
        pub fun removeKeys (pks: [String]);
    }
    
    // ------- Structs -------
    pub struct PubKeyAttr {
        pub let sigAlgo: UInt8;
        pub let weight: UFix64
        
        init(sa: UInt8, w: UFix64) {
            self.sigAlgo = sa;
            self.weight = w;
        }
    }

    
    // ------- Resources ------- 
    pub resource PayloadDetails {
        pub var txIndex: UInt64;
        pub var method: String;
        pub(set) var rsc: @AnyResource?;
        access(self) let args: [AnyStruct];
        
        access(contract) let signatures: [[UInt8]];
        access(contract) let pubKeys: [String];
        
        pub fun getArg(i: UInt): AnyStruct? {
            return self.args[i]
        }      

        // Calculate the bytes of a payload
        pub fun getSignableData(): [UInt8] {
            var s = self.txIndex.toBigEndianBytes();
            s = s.concat(self.method.utf8);
            var i: Int = 0;
            while i < self.args.length {
                let a = self.args[i];
                var b: [UInt8] = [];
                let t = a.getType();
                switch t {
                    case Type<String>():
                        let temp = a as? String;
                        b = temp!.utf8; 
                    case Type<UInt64>():
                        let temp = a as? UInt64;
                        b = temp!.toBigEndianBytes(); 
                    case Type<UFix64>():
                        let temp = a as? UFix64;
                        b = temp!.toBigEndianBytes(); 
                    case Type<UInt8>():
                        let temp = a as? UInt8;
                        b = temp!.toBigEndianBytes();
                    case Type<Address>():
                        let temp = a as? Address;
                        b = temp!.toBytes();
                    case Type<Path>():
                        b = "Path:".concat(i.toString()).utf8;
                    case Type<StoragePath>():
                        b = "StoragePath:".concat(i.toString()).utf8;
                    case Type<PrivatePath>():
                        b = "PrivatePath:".concat(i.toString()).utf8;
                    case Type<PublicPath>():
                        b = "PublicPath:".concat(i.toString()).utf8;
                    default:
                        panic ("Payload arg type not supported")
                }
                s = s.concat(b);
                i = i + 1
            }
            return s; 
        }
        
        // Verify the signature and return the total weight of valid signatures, if any.
        pub fun verifySigners (pks: [String], sigs: [[UInt8]], currentKeyList: {String: PubKeyAttr}): UFix64? {
            assert(pks.length == sigs.length, message: "Cannot verify signatures without corresponding public keys");
            
            var totalAuthorizedWeight: UFix64 = 0.0;
            var keyList = Crypto.KeyList();
            let keyListSignatures: [Crypto.KeyListSignature] = []
            var payloadInBytes: [UInt8] = self.getSignableData();

            // index of the public keys and signature list
            var i = 0;
            var keyIndex = 0;
            while (i < pks.length) {
                // check if the public key is a registered signer
                if (currentKeyList[pks[i]] == nil){
                    i = i + 1;
                    continue;
                }

                let pk = PublicKey(
                    publicKey: pks[i].decodeHex(),
                    signatureAlgorithm: SignatureAlgorithm(rawValue: currentKeyList[pks[i]]!.sigAlgo) ?? panic ("Invalid signature algo")
                )
                
                let keyListSig = Crypto.KeyListSignature(keyIndex: keyIndex, signature: sigs[i]);
                keyListSignatures.append(keyListSig);

                keyList.add(
                    pk, 
                    hashAlgorithm: HashAlgorithm.SHA3_256,
                    weight: currentKeyList[pks[i]]!.weight
                )
                totalAuthorizedWeight = totalAuthorizedWeight + currentKeyList[pks[i]]!.weight
                i = i + 1;
                keyIndex = keyIndex + 1;
            }
            
            let isValid = keyList.verify(
                signatureSet: keyListSignatures,
                signedData: payloadInBytes,
            )
            if (isValid) {
                return totalAuthorizedWeight
            } else {
                return nil
            }
        }
        
        pub fun addSignature(sig: [UInt8], publicKey: String){
            self.signatures.append(sig);
            self.pubKeys.append(publicKey);
        }
        
        destroy () {
            destroy self.rsc
        }

        init(txIndex: UInt64, method: String, args: [AnyStruct], rsc: @AnyResource?) {
            self.args = args;
            self.txIndex = txIndex;
            self.method = method;
            self.signatures= []
            self.pubKeys = []
            
            let r: @AnyResource <- rsc ?? nil
            if r != nil && r.isInstance(Type<@FungibleToken.Vault>()) {
                let vault <- r as! @FungibleToken.Vault
                assert(vault.balance == args[0] as! UFix64, message: "First argument must be balance of Vault")
                self.rsc <- vault;
            } else {
                self.rsc <- r;
            }
        }
    }
    
    pub resource Manager: SignatureManager {
        
        // Sequential identifier for each stored tx payload.
        pub var txIndex: UInt64;
        // Map of {publicKey: PubKeyAttr}
        access(self) let keyList: {String: PubKeyAttr};
        // Map of {txIndex: PayloadDetails}
        access(self) let payloads: @{UInt64: PayloadDetails}

        pub fun getSignerKeys(): [String] {
            return self.keyList.keys
        }

        pub fun getSignerKeyAttr(publicKey: String): PubKeyAttr? {
            return self.keyList[publicKey]
        }
        
        pub fun removePayload(txIndex: UInt64): @PayloadDetails {
            assert(self.payloads.containsKey(txIndex), message: "no payload at txIndex")
            return <- self.payloads.remove(key: txIndex)!
        }
        
        pub fun configureKeys (pks: [String], kws: [UFix64], sa: [UInt8]) {
            var i: Int =  0;
            while (i < pks.length) {
                let a = PubKeyAttr(sa: sa[i], w: kws[i])
                self.keyList.insert(key: pks[i], a)
                i = i + 1;
            }
        }

        pub fun removeKeys (pks: [String]) {
            var i: Int =  0;
            while (i < pks.length) {
                self.keyList.remove(key:pks[i])
                i = i + 1;
            }
        }
        
        pub fun addNewPayload (resourceId: UInt64, payload: @PayloadDetails, publicKey: String, sig: [UInt8]) {

            // Reject the tx if the provided key is not in the keyList
            assert(self.keyList.containsKey(publicKey), message: "Public key is not a registered signer");

            // Ensure the signed txIndex is the next txIndex for this resource
            let txIndex = self.txIndex + (1 as UInt64);
            assert(payload.txIndex == txIndex, message: "Incorrect txIndex provided in payload")
            assert(!self.payloads.containsKey(txIndex), message: "Payload index already exist");
            self.txIndex = txIndex;

            // Check if the payloadSig is signed by one of the keys in `keyList`, preventing others from adding to storage
            // if approvalWeight is nil, the public key is not in the `keyList` or cannot be verified
            let approvalWeight = payload.verifySigners(pks: [publicKey], sigs: [sig], currentKeyList: self.keyList)
            if ( approvalWeight == nil) {
                panic ("Invalid signer")
            }
            
            // Insert the payload and the first signature into the resource maps
            payload.addSignature(sig: sig, publicKey: publicKey)
            self.payloads[txIndex] <-! payload;

            emit NewPayloadAdded(resourceId: resourceId, txIndex: txIndex)
        }

        pub fun addPayloadSignature (resourceId: UInt64, txIndex: UInt64, publicKey: String, sig: [UInt8]) {
            assert(self.payloads.containsKey(txIndex), message: "Payload has not been added");
            assert(self.keyList.containsKey(publicKey), message: "Public key is not a registered signer");

            let p <- self.payloads.remove(key: txIndex)!;
            let currentIndex = p.signatures.length
            var i = 0;
            while i < currentIndex {
                if p.pubKeys[i] == publicKey {
                    break
                }
                i = i + 1;
            } 
            if i < currentIndex {
                self.payloads[txIndex] <-! p;
                panic ("Signature already added for this txIndex")
            } else {
                let approvalWeight = p.verifySigners( pks: [publicKey], sigs: [sig], currentKeyList: self.keyList)
                if ( approvalWeight == nil) {
                    self.payloads[txIndex] <-! p;
                    panic ("Invalid signer")
                } else {
                    p.addSignature(sig: sig, publicKey: publicKey)
                    self.payloads[txIndex] <-! p;

                    emit NewPayloadSigAdded(resourceId: resourceId, txIndex: txIndex)
                }
            }

        }

        // Ensure the total weights of the tx signers is sufficient to execute the tx
        pub fun readyForExecution(txIndex: UInt64): @PayloadDetails? {
            assert(self.payloads.containsKey(txIndex), message: "No payload for such index");
            let p <- self.payloads.remove(key: txIndex)!;
            let approvalWeight = p.verifySigners( pks: p.pubKeys, sigs: p.signatures, currentKeyList: self.keyList)
            if (approvalWeight! >= 1000.0) {
                return <- p
            } else {
                self.payloads[txIndex] <-! p;
                return nil
            }
        }

        destroy () {
            destroy self.payloads
        }
        
        init(publicKeys: [String], pubKeyAttrs: [PubKeyAttr]){
            assert( publicKeys.length == pubKeyAttrs.length, message: "Public keys must have associated attributes")
            self.payloads <- {};
            self.keyList = {};
            self.txIndex = 0;
            
            var i: Int = 0;
            while (i < publicKeys.length){
                self.keyList.insert(key: publicKeys[i], pubKeyAttrs[i]);
                i = i + 1;
            }
        }
    }
        
    pub fun createMultiSigManager(publicKeys: [String], pubKeyAttrs: [PubKeyAttr]): @Manager {
        return <- create Manager(publicKeys: publicKeys, pubKeyAttrs: pubKeyAttrs)
    }

    pub fun createPayload(txIndex: UInt64, method: String, args: [AnyStruct], rsc: @AnyResource?): @PayloadDetails{
        return <- create PayloadDetails(txIndex: txIndex, method: method, args: args, rsc: <-rsc)
    }
}
