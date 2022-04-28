import FungibleToken from 0xf233dcee88fe0abe
import FlowStorageFees from 0xe467b9dd11fa00df

pub contract BloctoStorageRent {

  pub let BloctoStorageRentAdminStoragePath: StoragePath

  access(contract) var StorageRentRefillThreshold: UInt64

  access(contract) var RefilledAccounts: [Address]

  access(contract) var RefilledAccountInfos: {Address: RefilledAccountInfo}

  access(contract) var RefillRequiredBlocks: UInt64

  pub fun getStorageRentRefillThreshold(): UInt64 {
    return self.StorageRentRefillThreshold
  }

  pub fun getRefilledAccounts(): [Address] {
    return self.RefilledAccounts
  }

  pub fun getRefilledAccountInfos(): {Address: RefilledAccountInfo} {
    return self.RefilledAccountInfos
  }

  pub fun getRefillRequiredBlocks(): UInt64 {
    return self.RefillRequiredBlocks
  }

  pub fun tryRefill(_ address: Address) {
    self.cleanExpiredRefilledAccounts(10)

    let recipient = getAccount(address)
    let receiverRef = recipient.getCapability(/public/flowTokenReceiver).borrow<&{FungibleToken.Receiver}>()
    if receiverRef == nil || receiverRef!.owner == nil {
      return
    }

    if self.RefilledAccountInfos[address] != nil && getCurrentBlock().height - self.RefilledAccountInfos[address]!.atBlock < self.RefillRequiredBlocks {
       return
    }

    var low: UInt64 = recipient.storageUsed
    var high: UInt64 = recipient.storageCapacity
    if high < low {
      high <-> low
    }

    if high - low < self.StorageRentRefillThreshold {
      let vaultRef = self.account.borrow<&{FungibleToken.Provider}>(from: /storage/flowTokenVault)
      if vaultRef == nil {
        return
      }

      let requiredAmount = FlowStorageFees.storageCapacityToFlow(FlowStorageFees.convertUInt64StorageBytesToUFix64Megabytes(self.StorageRentRefillThreshold * 2))
      receiverRef!.deposit(from: <-vaultRef!.withdraw(amount: requiredAmount))
      self.addRefilledAccount(address)
    }
  }

  pub fun checkEligibility(_ address: Address): Bool {
    if self.RefilledAccountInfos[address] != nil && getCurrentBlock().height - self.RefilledAccountInfos[address]!.atBlock < self.RefillRequiredBlocks {
       return false
    }
    let acct = getAccount(address)
    var high: UInt64 = acct.storageCapacity
    var low: UInt64 = acct.storageUsed
    if high < low {
      high <-> low
    }

    if high - low >= self.StorageRentRefillThreshold {
      return false
    }

    return true
  }

  access(contract) fun addRefilledAccount(_ address: Address) {
    if self.RefilledAccountInfos[address] != nil {
      self.RefilledAccounts.remove(at: self.RefilledAccountInfos[address]!.index)
    }

    self.RefilledAccounts.append(address)
    self.RefilledAccountInfos[address] = RefilledAccountInfo(self.RefilledAccounts.length-1, getCurrentBlock().height)
  }

  pub fun cleanExpiredRefilledAccounts(_ batchSize: Int) {
    var index = 0
    while index < batchSize && self.RefilledAccounts.length > index {
      if self.RefilledAccountInfos[self.RefilledAccounts[index]] != nil &&
        getCurrentBlock().height - self.RefilledAccountInfos[self.RefilledAccounts[index]]!.atBlock < self.RefillRequiredBlocks {
        break
      }

      self.RefilledAccountInfos.remove(key: self.RefilledAccounts[index])
      self.RefilledAccounts.remove(at: index)
      index = index + 1
    }
  }

  pub struct RefilledAccountInfo {
    pub let atBlock: UInt64
    pub let index: Int

    init(_ index: Int, _ atBlock: UInt64) {
      self.index = index
      self.atBlock = atBlock
    }
  }

  pub resource Admin {
    pub fun setStorageRentRefillThreshold(_ threshold: UInt64) {
      BloctoStorageRent.StorageRentRefillThreshold = threshold
    }

    pub fun setRefillRequiredBlocks(_ blocks: UInt64) {
      BloctoStorageRent.RefillRequiredBlocks = blocks
    }
  }

  init() {
    self.BloctoStorageRentAdminStoragePath = /storage/BloctoStorageRentAdmin
    self.StorageRentRefillThreshold = 5000
    self.RefilledAccounts = []
    self.RefilledAccountInfos = {}
    self.RefillRequiredBlocks = 86400

    let admin <- create Admin()
    self.account.save(<-admin, to: self.BloctoStorageRentAdminStoragePath)
  }
}
