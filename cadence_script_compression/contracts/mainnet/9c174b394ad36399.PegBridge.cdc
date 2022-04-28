/// PegBridge support mint via co-signed message, and burn, will call corresponding PegToken contract
/// Account of this contract must has Minter/Burner resource for corresponding PegToken
/// interfaces/resources in FTMinterBurner are needed to avoid token specific types
import FungibleToken from 0xf233dcee88fe0abe
import cBridge from 0x9c174b394ad36399
import PbPegged from 0x9c174b394ad36399
import DelayedTransfer from 0x9c174b394ad36399
// FTMinterBurner is needed for mint/burn
import FTMinterBurner from 0x9c174b394ad36399

pub contract PegBridge {
  // path for admin resource
  pub let AdminPath: StoragePath
  // path for FTMinterBurnerMap resource
  pub let FTMBMapPath: StoragePath

  // ========== events ==========
  pub event Mint(
    mintId: String,
    receiver: Address,
    token: String,
    amount: UFix64,
    refChId: UInt64,
    refId: String,
    depositor: String
  )

  pub event Burn(
    burnId: String,
    burner: Address,
    token: String,
    amount: UFix64,
    toChain: UInt64,
    toAddr: String
  )

  // ========== structs ==========
  // token vault type identifier string to its config so we can borrow to deposit minted token
  pub struct TokenCfg {
    pub let vaultPub: PublicPath
    pub let minBurn: UFix64
    pub let maxBurn: UFix64
    // if mint amount > delayThreshold, put into delayed transfer map
    pub let delayThreshold: UFix64

    init(vaultPub:PublicPath, minBurn: UFix64, maxBurn: UFix64, delayThreshold: UFix64) {
      self.vaultPub = vaultPub
      self.minBurn = minBurn
      self.maxBurn = maxBurn
      self.delayThreshold = delayThreshold
    }
  }

  // info about one user burn
  pub struct BurnInfo {
    pub let amt: UFix64
    pub let toChId: UInt64
    pub let toAddr: String
    pub let nonce: UInt64

    init(amt: UFix64, toChId: UInt64, toAddr: String, nonce: UInt64) {
      self.amt = amt
      self.toChId = toChId
      self.toAddr = toAddr
      self.nonce = nonce
    }
  }

  // ========== contract states and maps ==========
  // unique chainid required by cbridge system
  pub let chainID: UInt64
  // domainPrefix to ensure no replay on co-sign msgs
  access(contract) let domainPrefix: [UInt8]
  // similar to solidity pausable
  pub var isPaused: Bool

  // key is token vault identifier, eg. A.1122334455667788.ExampleToken.Vault
  access(account) var tokMap: {String: TokenCfg}
  // save for each mint/burn to avoid duplicated process
  // key is calculated mintID or burnID
  access(account) var records: {String: Bool}

  pub fun getTokenConfig(identifier: String): TokenCfg {
      let tokenCfg = self.tokMap[identifier]!
      return tokenCfg
  }

  pub fun recordExist(id: String): Bool {
      return self.records.containsKey(id)
  }

  // ========== resources ==========
  pub resource PegBridgeAdmin {
    pub fun addTok(identifier: String, tok: TokenCfg) {
      assert(!PegBridge.tokMap.containsKey(identifier), message: "this token already exist")
      PegBridge.tokMap[identifier] = tok
    }

    pub fun rmTok(identifier: String) {
      assert(PegBridge.tokMap.containsKey(identifier), message: "this token do not exist")
      PegBridge.tokMap.remove(key: identifier)
    }

    pub fun pause() {
      PegBridge.isPaused = true
      DelayedTransfer.pause()
    }
    pub fun unPause() {
      PegBridge.isPaused = false
      DelayedTransfer.unPause()
    }

    pub fun createPegBridgeAdmin(): @PegBridgeAdmin {
        return <-create PegBridgeAdmin()
    }
  }

  // token admin must create minter/burner resource and call add
  pub resource interface IAddMinter {
    pub fun addMinter(minter: @FTMinterBurner.Minter)
  }
  pub resource interface IAddBurner {
    pub fun addBurner(burner: @FTMinterBurner.Burner)
  }

  /// MinterBurnerMap support public add minter/burner by token admin,
  /// del minter/burner by account, and mint/burn corresponding ft
  /// when called by this contract
  pub resource MinterBurnerMap: IAddMinter, IAddBurner {
    // map from token vault identifier to minter or burner resource
    access(account) var hasMinters: @{String: FTMinterBurner.Minter}
    access(account) var hasBurners: @{String: FTMinterBurner.Burner}

    // called by token admin
    pub fun addMinter(minter: @FTMinterBurner.Minter) {
      let idStr = minter.getType().identifier
      // TODO, we use this method to remove "Minter" to "Vault", maybe better way
      let newIdStr = idStr.slice(from: 0, upTo: idStr.length - 6).concat("Vault")
      // only supported token minter can be added
      assert(PegBridge.tokMap.containsKey(newIdStr), message: "this token not support")

      let oldMinter <- self.hasMinters[newIdStr] <- minter
      destroy oldMinter
    }
    pub fun addBurner(burner: @FTMinterBurner.Burner) {
      let idStr = burner.getType().identifier
      let newIdStr = idStr.slice(from: 0, upTo: idStr.length - 6).concat("Vault")
      // only supported token burner can be added
      assert(PegBridge.tokMap.containsKey(newIdStr), message: "this token not support")
      let old <- self.hasBurners[newIdStr] <- burner
      destroy old
    }

    // only account can call this as not exposed by public path, other contracts under
    // same account can also call
    access(account) fun delMinter(idStr: String) {
      let minter <- self.hasMinters.remove(key: idStr) ?? panic("missing Minter")
      destroy minter
    }
    access(account) fun delBurner(idStr: String) {
      let burner <- self.hasBurners.remove(key: idStr) ?? panic("missing Burner")
      destroy burner
    }

    // for extra security, only this contract can call mint/burn
    access(contract) fun mint(id:String, amt: UFix64): @FungibleToken.Vault {
      let minter = &self.hasMinters[id] as &FTMinterBurner.Minter
      return <- minter.mintTokens(amount: amt)
    }
    access(contract) fun burn(id:String, from: @FungibleToken.Vault) {
      let burner = &self.hasBurners[id] as &FTMinterBurner.Burner
      burner.burnTokens(from: <- from)
    }
    init(){
      self.hasMinters <- {}
      self.hasBurners <- {}
    }
    destroy() {
      destroy self.hasMinters
      destroy self.hasBurners
    }
  }

  // ========== functions ==========
  init(chID:UInt64) {
    self.chainID = chID
    // domainPrefix is chainID big endianbytes followed by "A.xxxxxx.PegBridge".utf8, xxxx is this contract account
    self.domainPrefix = chID.toBigEndianBytes().concat(self.getType().identifier.utf8)
    self.isPaused = false

    self.records = {}
    self.tokMap = {}

    self.AdminPath = /storage/PegBridgeAdmin
    self.account.save<@PegBridgeAdmin>(<- create PegBridgeAdmin(), to: self.AdminPath)

    self.FTMBMapPath = /storage/FTMinterBurnerMap
    // needed for minter/burner
    self.account.save(<-create MinterBurnerMap(), to: self.FTMBMapPath)
    // anyone can call /public/AddMinter to add a minter to map
    self.account.link<&MinterBurnerMap{IAddMinter}>(/public/AddMinter, target: self.FTMBMapPath)
    self.account.link<&MinterBurnerMap{IAddBurner}>(/public/AddBurner, target: self.FTMBMapPath)
  }

  pub fun mint(token: String, pbmsg: [UInt8], sigs: [cBridge.SignerSig]) {
    pre {
      !self.isPaused: "contract is paused"
    }
    let domain = self.domainPrefix.concat("Mint".utf8)
    assert(cBridge.verify(data: domain.concat(pbmsg), sigs: sigs), message: "verify sigs failed")
    let mintInfo = PbPegged.Mint(pbmsg)
    assert(mintInfo.eqToken(tkStr: token), message: "mismatch token string")

    let tokCfg = PegBridge.tokMap[token] ?? panic("token not support in contract")
    let mintId = String.encodeHex(HashAlgorithm.SHA3_256.hash(pbmsg))
    assert(!self.records.containsKey(mintId), message: "mintId already exists")
    self.records[mintId] = true

    let receiverCap = getAccount(mintInfo.receiver).getCapability<&{FungibleToken.Receiver}>(tokCfg.vaultPub)
    let minterMap = self.account.borrow<&MinterBurnerMap>(from: self.FTMBMapPath)!
    let mintedVault: @FungibleToken.Vault <- minterMap.mint(id: token, amt: mintInfo.amount)
    
    if mintInfo.amount > tokCfg.delayThreshold {
      // add to delayed xfer
      DelayedTransfer.addDelayXfer(id: mintId, receiverCap: receiverCap, from: <- mintedVault)
    } else {
      let receiverRef = receiverCap.borrow() ?? panic("Could not borrow a reference to the receiver")
      receiverRef.deposit(from: <- mintedVault)
    }

    emit Mint(
      mintId: mintId,
      receiver: mintInfo.receiver,
      token: token,
      amount: mintInfo.amount,
      refChId: mintInfo.refChainId,
      refId: mintInfo.refId,
      depositor: mintInfo.depositor
    )
  }
  // 
  pub fun burn(from: &AnyResource{FungibleToken.Provider}, info:BurnInfo) {
    pre {
      !self.isPaused: "contract is paused"
    }
    let user = from.owner!.address
    let tokStr = from.getType().identifier
    let tokenCfg = self.tokMap[tokStr]!
    assert(info.amt >= tokenCfg.minBurn, message: "burn amount less than min burn")
    if tokenCfg.maxBurn > 0.0 {
      assert(info.amt < tokenCfg.maxBurn, message: "burn amount larger than max burn")
    }
    // calculate burnId
    let concatStr = user.toString().concat(tokStr).concat(info.amt.toString()).concat(info.nonce.toString())
    let burnId = String.encodeHex(HashAlgorithm.SHA3_256.hash(concatStr.utf8))
    assert(!self.records.containsKey(burnId), message: "burnId already exists")
    self.records[burnId] = true

    let mbMap = self.account.borrow<&MinterBurnerMap>(from: self.FTMBMapPath)!
    let burnVault <-from.withdraw(amount: info.amt)
    mbMap.burn(id: tokStr, from: <-burnVault)

    emit Burn(
     burnId: burnId,
     burner: user,
     token: tokStr,
     amount: info.amt,
     toChain: info.toChId,
     toAddr: info.toAddr
    )
  }
  // large amount mint
  pub fun executeDelayedTransfer(mintId: String) {
    pre {
      !self.isPaused: "contract is paused"
    }
    DelayedTransfer.executeDelayXfer(mintId)
  }
}