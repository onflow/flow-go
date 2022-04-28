import Crypto

import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61

pub contract ColdStorage {

  pub struct Key {
    pub let publicKey: [UInt8]
    pub let signatureAlgorithm: UInt8
    pub let hashAlgorithm: UInt8

    init(
      publicKey: [UInt8],
      signatureAlgorithm: SignatureAlgorithm,
      hashAlgorithm: HashAlgorithm
    ) {
      self.publicKey = publicKey
      self.signatureAlgorithm = signatureAlgorithm.rawValue
      self.hashAlgorithm = hashAlgorithm.rawValue
    }
  }

  pub struct interface ColdStorageRequest {
    pub var signature: Crypto.KeyListSignature
    pub var seqNo: UInt64
    pub var spenderAddress: Address

    pub fun signableBytes(): [UInt8]
  }

  pub struct WithdrawRequest: ColdStorageRequest {
    pub var signature: Crypto.KeyListSignature
    pub var seqNo: UInt64

    pub var spenderAddress: Address
    pub var recipientAddress: Address
    pub var amount: UFix64

    init(
      spenderAddress: Address,
      recipientAddress: Address,
      amount: UFix64,
      seqNo: UInt64,
      signature: Crypto.KeyListSignature,
    ) {
      self.spenderAddress = spenderAddress
      self.recipientAddress = recipientAddress
      self.amount = amount

      self.seqNo = seqNo
      self.signature = signature
    }

    pub fun signableBytes(): [UInt8] {
      let spenderAddress = self.spenderAddress.toBytes()
      let recipientAddressBytes = self.recipientAddress.toBytes()
      let amountBytes = self.amount.toBigEndianBytes()
      let seqNoBytes = self.seqNo.toBigEndianBytes()

      return spenderAddress.concat(recipientAddressBytes).concat(amountBytes).concat(seqNoBytes)
    }
  }

  pub resource PendingWithdrawal {

    access(self) var pendingVault: @FungibleToken.Vault
    access(self) var request: WithdrawRequest

    init(pendingVault: @FungibleToken.Vault, request: WithdrawRequest) {
      self.pendingVault <- pendingVault
      self.request = request
    }

    pub fun execute(fungibleTokenReceiverPath: PublicPath) {
      var pendingVault <- FlowToken.createEmptyVault()
      self.pendingVault <-> pendingVault

      let recipient = getAccount(self.request.recipientAddress)
      let receiver = recipient
        .getCapability(fungibleTokenReceiverPath)!
        .borrow<&{FungibleToken.Receiver}>()
        ?? panic("Unable to borrow receiver reference for recipient")

      receiver.deposit(from: <- pendingVault)
    }

    destroy (){
      pre {
        self.pendingVault.balance == 0.0 as UFix64
      }
      destroy self.pendingVault
    }
  }

  pub resource interface PublicVault {
    pub fun getSequenceNumber(): UInt64

    pub fun getBalance(): UFix64

    pub fun getKey(): Key

    pub fun prepareWithdrawal(request: WithdrawRequest): @PendingWithdrawal
  }

  pub resource Vault : FungibleToken.Receiver, PublicVault {
    access(self) var address: Address
    access(self) var key: Key
    access(self) var contents: @FungibleToken.Vault
    access(self) var seqNo: UInt64

    pub fun deposit(from: @FungibleToken.Vault) {
      self.contents.deposit(from: <-from)
    }

    pub fun getSequenceNumber(): UInt64 {
        return self.seqNo
    }

    pub fun getBalance(): UFix64 {
      return self.contents.balance
    }

    pub fun getKey(): Key {
      return self.key
    }

    pub fun prepareWithdrawal(request: WithdrawRequest): @PendingWithdrawal {
      pre {
        self.isValidSignature(request: request)
      }
      post {
        self.seqNo == request.seqNo + UInt64(1)
      }

      self.incrementSequenceNumber()

      return <- create PendingWithdrawal(pendingVault: <- self.contents.withdraw(amount: request.amount), request: request)
    }

    access(self) fun incrementSequenceNumber(){
      self.seqNo = self.seqNo + UInt64(1)
    }

    access(self) fun isValidSignature(request: {ColdStorage.ColdStorageRequest}): Bool {
      pre {
        self.seqNo == request.seqNo
        self.address == request.spenderAddress
      }

      return ColdStorage.validateSignature(
        key: self.key,
        signature: request.signature,
        message: request.signableBytes()
      )
    }

    init(address: Address, key: Key, contents: @FungibleToken.Vault) {
      self.key = key
      self.seqNo = UInt64(0)
      self.contents <- contents
      self.address = address
    }

    destroy() {
      destroy self.contents
    }
  }

  pub fun createVault(
    address: Address,
    key: Key,
    contents: @FungibleToken.Vault,
  ): @Vault {
    return <- create Vault(address: address, key: key, contents: <- contents)
  }

  pub fun validateSignature(
    key: Key,
    signature: Crypto.KeyListSignature,
    message: [UInt8],
  ): Bool {
    let keyList = Crypto.KeyList()

    let signatureAlgorithm = SignatureAlgorithm(rawValue: key.signatureAlgorithm) ?? panic("invalid signature algorithm")
    let hashAlgorithm = HashAlgorithm(rawValue: key.hashAlgorithm)  ?? panic("invalid hash algorithm")

    keyList.add(
      PublicKey(
        publicKey: key.publicKey,
        signatureAlgorithm: signatureAlgorithm,
      ),
      hashAlgorithm: hashAlgorithm,
      weight: 1000.0,
    )

    return keyList.verify(
      signatureSet: [signature],
      signedData: message
    )
  }
}
