import FungibleToken from 0xf233dcee88fe0abe
import TeleportedTetherToken from 0xcfdd90d4a00f7b5b
import BloctoToken from 0x0f9df91c9121c460

pub contract BloctoIdo {

  pub let BloctoIdoAdminStoragePath: StoragePath
  pub let BloctoIdoAdminPublicPath: PublicPath

  pub let BloctoIdoUserStoragePath: StoragePath
  pub let BloctoIdoUserPublicPath: PublicPath

  pub event AddKycInfo(name: String, addr: Address)
  pub event AddKycInfoError(name: String, addr: Address, reason: String)
  pub event SelectPool(name: String, addr: Address, poolType: String)
  pub event Deposit(name: String, addr: Address, amount: UFix64)
  pub event UpdateQuota(name: String, addr: Address, amount: UFix64)
  pub event UpdateQuotaError(name: String, addr: Address, reason: String)
  pub event Distribute(name: String, addr: Address, amount: UFix64)
  pub event DistributingError(name: String, addr: Address, reason: String)

  access(contract) var fee: @BloctoToken.Vault

  access(contract) var vault: @TeleportedTetherToken.Vault

  access(contract) var activities: {String: Activity}

  access(contract) var activitiesOrder: [String]

  access(contract) var idToAddress: {UInt64: Address}

  pub var userId: UInt64

  pub struct TokenInfo {
    pub(set) var contractName: String
    pub(set) var address: Address
    pub(set) var storagePath: StoragePath
    pub(set) var receiverPath: PublicPath
    pub(set) var balancePath: PublicPath

    init() {
      self.contractName = ""
      self.address = 0x0
      self.storagePath = /storage/defaultTokenPath
      self.receiverPath = /public/defaultTokenPath
      self.balancePath = /public/defaultTokenPath
    }
  }

  pub struct PoolConfig {
    pub(set) var amount: UFix64
    pub(set) var minimumStake: UFix64
    pub(set) var upperBound: UFix64
    pub(set) var selectFee: UFix64
    pub(set) var exchangeRate: UFix64

    init() {
      self.amount = 0.0
      self.minimumStake = 0.0
      self.upperBound = 0.0
      self.selectFee = 0.0
      self.exchangeRate = 0.0
    }
  }

  pub struct ActivityConfig {
    pub var tokenInfo: BloctoIdo.TokenInfo
    pub var schedule: {String: [UFix64]}
    pub var poolConfig: {String: BloctoIdo.PoolConfig}
    pub var snapshotTimes: UInt64

    init(
      _ tokenInfo: BloctoIdo.TokenInfo,
      _ schedule: {String: [UFix64]},
      _ poolConfig: {String: BloctoIdo.PoolConfig},
      _ snapshotTimes: UInt64
    ) {
      self.tokenInfo = tokenInfo
      self.schedule = schedule
      self.poolConfig = poolConfig
      self.snapshotTimes = snapshotTimes
    }
  }

  pub struct Activity {
    // activity config
    pub var tokenInfo: TokenInfo
    // TODO use enum
    // Key => KYC | SELECT_POOL | DEPOSIT | DISTRIBUTE
    // Value => [startTimestamp, endTimestamp]
    pub var schedule: {String: [UFix64]}
    pub var poolConfig: {String: PoolConfig}

    // user data
    pub var kycList: [Address]
    pub var userInfos: {Address: UserInfo}
    pub var totalValidStake: {String: UFix64}
    pub(set) var snapshotTimes: UInt64

    init() {
      self.tokenInfo = TokenInfo()
      self.schedule = {}
      self.poolConfig = {}
      self.kycList = []
      self.userInfos = {}
      self.totalValidStake = {}
      self.snapshotTimes = 0
    }
  }

  pub struct StakeInfo {
    pub(set) var stakeAmount: UFix64
    pub(set) var lockAmount: UFix64

    init() {
      self.stakeAmount = 0.0
      self.lockAmount = 0.0
    }
  }

  pub struct UserInfo {
    // TODO use enum
    // LIMITED | UNLIMITED
    pub(set) var poolType: String
    pub(set) var stakeInfos: [StakeInfo]

    pub(set) var quota: UFix64
    pub(set) var deposited: UFix64
    pub(set) var distributed: Bool

    init() {
      self.poolType = ""
      self.stakeInfos = []
      self.quota = 0.0
      self.deposited = 0.0
      self.distributed = false
    }
  }

  pub resource interface UserPublic {
    pub let id: UInt64
  }

  pub resource User: UserPublic {
    pub let id: UInt64

          init(id: UInt64) {
              self.id = id
          }

    pub fun selectPool(name: String, poolType: String, vault: @BloctoToken.Vault) {
      let activity = BloctoIdo.activities[name] ?? panic("ido does not exist")
      let interval = activity.schedule["SELECT_POOL"] ?? panic("stage does not exist")
      let current = getCurrentBlock().timestamp
      if current < interval[0] || current > interval[1] {
        panic("stage closed")
      }

      let address = BloctoIdo.idToAddress[self.id] ?? panic("invalid id")
      let userInfo = activity.userInfos[address] ?? panic("user info does not exist")
      if userInfo.poolType != "" {
        panic("you have already selected a pool")
      }

      let poolConfig = activity.poolConfig[poolType] ?? panic("unsupported pool type")
      if vault.balance < poolConfig.selectFee {
        panic("fee is not enough")
      }

      // calculate stake amount
      let validStake = BloctoIdo.calculateValidStake(poolType: poolType, stakeInfos: userInfo.stakeInfos, upperBound: poolConfig.upperBound)

      var oriTotalValidStake = 0.0
      if activity.totalValidStake[poolType] != nil {
        oriTotalValidStake = activity.totalValidStake[poolType]!
      }

      userInfo.poolType = poolType
      activity.userInfos[address] = userInfo
      activity.totalValidStake[poolType] = oriTotalValidStake + validStake
      BloctoIdo.activities[name] = activity

      emit SelectPool(name: name, addr: address, poolType: poolType)

      BloctoIdo.fee.deposit(from: <-vault);
    }

    pub fun deposit(name: String, vault: @TeleportedTetherToken.Vault) {
      let activity = BloctoIdo.activities[name] ?? panic("ido does not exist")
      let interval = activity.schedule["DEPOSIT"] ?? panic("stage does not exist")
      let current = getCurrentBlock().timestamp
      if current < interval[0] || current > interval[1] {
        panic("stage closed")
      }

      let addressOpt = self.owner
      if addressOpt == nil {
        panic("resource owner is nil")
      }
      let address = addressOpt!.address

      let expectedAddress = BloctoIdo.idToAddress[self.id] ?? panic("can't get expected address")
      if address != expectedAddress {
        panic("unexpected address")
      }

      let userInfo = activity.userInfos[address] ?? panic("user info does not exist")

      if (userInfo.deposited + vault.balance) > userInfo.quota {
        panic("insufficient quota")
      }

      userInfo.deposited = userInfo.deposited + vault.balance
      activity.userInfos[address] = userInfo
      BloctoIdo.activities[name] = activity

      emit Deposit(name: name, addr: address, amount: vault.balance)

      BloctoIdo.vault.deposit(from: <-vault);
    }
      }

  pub resource Admin {

    pub fun upsertActivity(_ name: String, _ activity: Activity) {
      if BloctoIdo.activities[name] == nil {
        BloctoIdo.activitiesOrder.append(name)
      }
      BloctoIdo.activities[name] = activity
    }

    pub fun removeActivity(_ name: String) {
      var idx = 0
      while idx < BloctoIdo.activitiesOrder.length {
        let activityName = BloctoIdo.activitiesOrder[idx]
        if activityName == name {
          BloctoIdo.activitiesOrder.remove(at: idx)
          break
        }
        idx = idx + 1
      }
      BloctoIdo.activities.remove(key: name)
    }

    pub fun addKycList(_ name: String, _ addrList: [Address]) {
      if !BloctoIdo.activities.containsKey(name) {
        panic("activity doesn't exist")
      }

      for addr in addrList {
        // check ido user
        let idoUserPublicRefOpt = getAccount(addr)
          .getCapability(BloctoIdo.BloctoIdoUserPublicPath)
          .borrow<&{BloctoIdo.UserPublic}>()
        if idoUserPublicRefOpt == nil {
          emit AddKycInfoError(name: name, addr: addr, reason: "failed to get ido user public")
          continue
        }
        let idoUserPublicRef = idoUserPublicRefOpt!

        // continue if already existed
        if BloctoIdo.activities[name]!.userInfos.containsKey(addr) {
          continue
        }

        let userInfo = BloctoIdo.UserInfo()
        BloctoIdo.activities[name]!.userInfos.insert(key: addr, userInfo)
        BloctoIdo.activities[name]!.kycList.append(addr)

        self.setIdToAddress(id: idoUserPublicRef.id, address: addr)

        emit AddKycInfo(name: name, addr: addr)
      }
    }

    pub fun addStakeInfo(name: String, addrList: [Address], stakeAmountList: [UFix64], lockAmountList: [UFix64]) {
      if !BloctoIdo.activities.containsKey(name) {
        panic("activity doesn't exist")
      }

      if addrList.length != stakeAmountList.length {
        panic("addrList.length != stakeAmountList.length")
      }

      if addrList.length != lockAmountList.length {
        panic("addrList.length != lockAmountList.length")
      }

      var idx = -1
      while true {
        idx = idx + 1
        if idx >= addrList.length {
          break
        }

        let addr = addrList[idx]
        if BloctoIdo.activities[name]!.userInfos.containsKey(addr) == nil {
          continue
        }

        let poolType = BloctoIdo.activities[name]!.userInfos[addr]!.poolType

        if poolType == "" {
          let stakeInfo = BloctoIdo.StakeInfo()
          stakeInfo.stakeAmount = stakeAmountList[idx]
          stakeInfo.lockAmount = lockAmountList[idx]
          BloctoIdo.activities[name]!.userInfos[addr]!.stakeInfos.append(stakeInfo)
        } else {
          let upperBound = BloctoIdo.activities[name]!.poolConfig[poolType]!.upperBound

          let preValidStake = BloctoIdo.calculateValidStake(
            poolType: poolType,
            stakeInfos: BloctoIdo.activities[name]!.userInfos[addr]!.stakeInfos,
            upperBound: upperBound
          )

          let stakeInfo = BloctoIdo.StakeInfo()
          stakeInfo.stakeAmount = stakeAmountList[idx]
          stakeInfo.lockAmount = lockAmountList[idx]
          BloctoIdo.activities[name]!.userInfos[addr]!.stakeInfos.append(stakeInfo)

          let postValidStake = BloctoIdo.calculateValidStake(
            poolType: poolType,
            stakeInfos: BloctoIdo.activities[name]!.userInfos[addr]!.stakeInfos,
            upperBound: upperBound
          )

          if BloctoIdo.activities[name]!.totalValidStake[poolType] != nil {
            BloctoIdo.activities[name]!.totalValidStake.insert(key: poolType, BloctoIdo.activities[name]!.totalValidStake[poolType]! - preValidStake + postValidStake)
          }
        }
      }
    }

    pub fun updateStakeInfo(name: String, addr: Address, stakeInfos: [StakeInfo]) {
      if !BloctoIdo.activities.containsKey(name) {
        panic("activity doesn't exist")
      }

      if BloctoIdo.activities[name]!.userInfos.containsKey(addr) == nil {
        panic("user doesn't exist")
      }
      let userInfo = BloctoIdo.activities[name]!.userInfos[addr]!

      if userInfo.poolType != "" {
        let upperBound = BloctoIdo.activities[name]!.poolConfig[userInfo.poolType]!.upperBound

        let userPreValidStake = BloctoIdo.calculateValidStake(
          poolType: userInfo.poolType,
          stakeInfos: userInfo.stakeInfos,
          upperBound: upperBound
        )

        let userPostValidStake = BloctoIdo.calculateValidStake(
          poolType: userInfo.poolType,
          stakeInfos: stakeInfos,
          upperBound: upperBound
        )

        var totalValidStake = 0.0
        if BloctoIdo.activities[name]!.totalValidStake[userInfo.poolType] != nil {
          totalValidStake = BloctoIdo.activities[name]!.totalValidStake[userInfo.poolType]!
          if totalValidStake >= userPreValidStake {
            totalValidStake = totalValidStake - userPreValidStake
          }
        }

        BloctoIdo.activities[name]!.totalValidStake.insert(
          key: userInfo.poolType,
          totalValidStake + userPostValidStake,
        )
      }

      userInfo.stakeInfos = stakeInfos
      BloctoIdo.activities[name]!.userInfos.insert(key: addr, userInfo)
    }

    pub fun updateQuota(name: String, addrList: [Address], quotaList: [UFix64]) {
      if !BloctoIdo.activities.containsKey(name) {
        panic("activity doesn't exist")
      }

      if addrList.length != quotaList.length {
        panic("addrList.length != quotaList.length")
      }

      var idx = -1
      while true {
        idx = idx + 1
        if idx >= addrList.length {
          break
        }

        let addr = addrList[idx]
        if !BloctoIdo.activities[name]!.userInfos.containsKey(addr) {
          emit UpdateQuotaError(name: name, addr: addr, reason: "doesn't exist user")
          continue
        }

        let userInfo = BloctoIdo.activities[name]!.userInfos[addr]!
        userInfo.quota = quotaList[idx]

        BloctoIdo.activities[name]!.userInfos.insert(key: addr, userInfo)
        emit UpdateQuota(name: name, addr: addr, amount: quotaList[idx])
      }
    }

    pub fun setIdToAddress(id: UInt64, address: Address) {
      BloctoIdo.idToAddress[id] = address
    }

    pub fun distribute(_ name: String, start: UInt64, num: UInt64) {
      let activity = BloctoIdo.activities[name] ?? panic("ido does not exist")
      let interval = activity.schedule["DISTRIBUTE"] ?? panic("stage does not exist")
      let current = getCurrentBlock().timestamp
      if current < interval[0] || current > interval[1] {
        panic("stage closed")
      }

      var curr = start
      let end = start + num
      while curr < end {
        let address = activity.kycList[curr]
        let userInfo = activity.userInfos[address] ?? panic("failed to get user info")

        if userInfo.distributed {
          curr = curr + 1
          continue
        }

        if userInfo.poolType != "" && userInfo.deposited != 0.0 {
          let poolConfig = activity.poolConfig[userInfo.poolType]
            ?? panic("invalid pool type")

          let vaultRef = BloctoIdo.account.borrow<&FungibleToken.Vault>(from: activity.tokenInfo.storagePath)
            ?? panic("failed to get vault")

          let receiverRefOpt = getAccount(address)
            .getCapability(activity.tokenInfo.receiverPath)
            .borrow<&{FungibleToken.Receiver}>()
          if receiverRefOpt == nil {
            emit DistributingError(name: name, addr: address, reason: "failed to borrow receiver")
            curr = curr + 1
            continue
          }

          let receiverRef = receiverRefOpt!
          let amount = userInfo.deposited / poolConfig.exchangeRate
          receiverRef.deposit(from: <-vaultRef.withdraw(amount: amount))
          BloctoIdo.activities[name] = activity
          emit Distribute(name: name, addr: address, amount: amount)
        }

        userInfo.distributed = true
        activity.userInfos[address] = userInfo

        curr = curr + 1
      }

      BloctoIdo.activities[name] = activity
    }

    pub fun withdrawFromFee(amount: UFix64): @BloctoToken.Vault {
      return <- (BloctoIdo.fee.withdraw(amount: amount) as! @BloctoToken.Vault)
    }

    pub fun withdrawFromVault(amount: UFix64): @TeleportedTetherToken.Vault {
      return <- (BloctoIdo.vault.withdraw(amount: amount) as! @TeleportedTetherToken.Vault)
    }
  }

  pub fun getActivity(_ name: String): Activity? {
    return BloctoIdo.activities[name]
  }

  pub fun getActivityConfig(_ name: String): ActivityConfig? {
    let activityOpt = BloctoIdo.activities[name]
    if activityOpt == nil {
      return nil
    }
    let activity = activityOpt!
    return ActivityConfig(activity.tokenInfo, activity.schedule, activity.poolConfig, activity.snapshotTimes)
  }

  pub fun getAddressById(id: UInt64): Address? {
    return BloctoIdo.idToAddress[id]
  }

  pub fun getActivityNames(): [String] {
    return BloctoIdo.activitiesOrder
  }

  pub fun createNewUser(): @User {
    let newUser <- create User(id: BloctoIdo.userId)
    BloctoIdo.userId =  BloctoIdo.userId + 1
    return <- newUser
  }

  pub fun calculateValidStake(poolType: String, stakeInfos: [StakeInfo], upperBound: UFix64): UFix64 {
    var lastEpochStakeAmount = 0.0
    var validStake = 0.0
    var weights = 1.0

    switch poolType {
    case "LIMITED":
      for stakeInfo in stakeInfos {
        var stakeAmount = stakeInfo.stakeAmount

        if stakeAmount < lastEpochStakeAmount {
          return 0.0
        }
        if upperBound != 0.0 && stakeAmount > upperBound {
          stakeAmount = upperBound
        }

        if stakeAmount > lastEpochStakeAmount {
          validStake = validStake * (weights - 1.0) / weights
          validStake = validStake + stakeAmount / weights
        }

        lastEpochStakeAmount = stakeAmount
        weights = weights + 1.0
      }
    case "UNLIMITED":
      for stakeInfo in stakeInfos {
        var stakeAmount = 0.0
        if stakeInfo.stakeAmount > stakeInfo.lockAmount {
          stakeAmount = stakeInfo.stakeAmount - stakeInfo.lockAmount
        }
        if stakeAmount < lastEpochStakeAmount {
          return 0.0
        }

        if stakeAmount > lastEpochStakeAmount {
          validStake = validStake * (weights - 1.0) / weights
          validStake = validStake + stakeAmount / weights
        }

        lastEpochStakeAmount = stakeAmount
        weights = weights + 1.0
      }
    }

    return validStake
  }

  pub fun getFeeBalance(): UFix64 {
    return BloctoIdo.fee.balance
  }

  pub fun getVaultBalance(): UFix64 {
    return BloctoIdo.vault.balance
  }

  init () {
    self.BloctoIdoAdminStoragePath = /storage/BloctoIdoAdmin
    self.BloctoIdoAdminPublicPath = /public/BloctoIdoAdmin

    self.BloctoIdoUserStoragePath = /storage/BloctoIdoUser
    self.BloctoIdoUserPublicPath = /public/BloctoIdoUser

    self.fee <- BloctoToken.createEmptyVault() as! @BloctoToken.Vault

    self.vault <- TeleportedTetherToken.createEmptyVault() as! @TeleportedTetherToken.Vault

    self.userId = 1
    self.activities = {}
    self.activitiesOrder = []
    self.idToAddress = {}

    let admin <- create Admin()
    self.account.save(<-admin, to: self.BloctoIdoAdminStoragePath)
  }
}
