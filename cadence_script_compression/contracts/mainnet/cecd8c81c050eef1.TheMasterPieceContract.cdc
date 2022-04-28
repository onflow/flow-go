/**
SPDX-FileCopyrightText: 2021 copyright 52a74d3b580cdb48eb87979860ca6efe <creator@themasterpiece.art>
SPDX-License-Identifier: GPL-3.0-or-later

## `TheMasterPieceContract` contract

The contract will reference TheMasterPixel owners
Allowing to get all addresses from everyone owning a collection of TheMasterPixel

## `TMPOwner`,  structure

Represents the pixel quantity by anyone owning a collection of TheMasterPixel
saleSize : owned pixels for sale in collection
walletSize: owned pixels in collection not for sale

## `TMPOwners`, `TMPSectorOwners` resources

TMPOwners will manage a dictionnary of TMPSectorOwners organized by sector
TMPSectorOwners will manage a dictionnary of TMPOwner oraganized by owner's address

TMPOwners
    {sector -> TMPSectorOwners}
        {address ->  TMPOwner}

## `TMPOwnersInterface` resource interfaces

Exposes functions to query address references and pixel quantity owned by sector

*/

pub contract TheMasterPieceContract {

  // Named Paths
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath

  init() {
    // Set our named paths
    self.CollectionStoragePath = /storage/TMPOwners
    self.CollectionPublicPath = /public/TMPOwners

    if (self.account.borrow<&TMPOwners>(from: self.CollectionStoragePath) == nil) {
      self.account.save(<-create TMPOwners(), to: self.CollectionStoragePath)
      self.account.link<&{TMPOwnersInterface}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
    }
  }

  access(account) fun setSaleSize(sectorId: UInt16, address: Address, size: UInt16) {
      let theOwnersRef = self.account.borrow<&TMPOwners>(from: /storage/TMPOwners)!
      theOwnersRef.setSaleSize(sectorId: sectorId, address: address, size: size)
  }

  access(account) fun setWalletSize(sectorId: UInt16, address: Address, size: UInt16) {
      let theOwnersRef = self.account.borrow<&TMPOwners>(from: /storage/TMPOwners)!
      theOwnersRef.setWalletSize(sectorId: sectorId, address: address, size: size)
  }

  pub fun getAddress(): Address {
    return self.account.address
  }

  // ########################################################################################

  pub struct TMPOwner {
      pub var saleSize: UInt16
      pub var walletSize: UInt16

      init() {
          self.saleSize = 0
          self.walletSize = 0
      }

      pub fun setSaleSize(saleSize: UInt16) {
          self.saleSize = saleSize
      }

      pub fun setWalletSize(walletSize: UInt16) {
          self.walletSize = walletSize
      }
  }

  pub resource TMPSectorOwners {
    priv var owners: {Address: TMPOwner}

    init() {
      self.owners = {}
    }

    priv fun addOwner(address: Address) {
      var owner = self.owners[address]
      if (owner == nil) {
        owner = TMPOwner()
        self.owners[address] = owner
      }
    }

    priv fun removeOwner(address: Address) {
      if (self.owners.containsKey(address) && (self.owners[address]!).walletSize == 0 && (self.owners[address]!).saleSize == 0) {
        self.owners.remove(key: address)
      }
    }

    access(contract) fun setWalletSize(address: Address, size: UInt16) {
      self.addOwner(address: address)
      (self.owners[address]!).setWalletSize(walletSize: size)
      self.removeOwner(address: address)
    }

    access(contract) fun setSaleSize(address: Address, size: UInt16) {
      (self.owners[address]!).setSaleSize(saleSize: size)
      self.removeOwner(address: address)
    }

    pub fun getOwners(): {Address: TMPOwner} {
      return self.owners
    }

    pub fun getOwner(address: Address): TMPOwner? {
      return self.owners[address]
    }

  }

  // ########################################################################################

  pub resource interface TMPOwnersInterface {
    pub fun getOwners(sectorId: UInt16): {Address: TMPOwner}
    pub fun getOwner(sectorId: UInt16, address: Address): TMPOwner?
    pub fun listSectors(): [UInt16]
  }

  pub resource TMPOwners: TMPOwnersInterface {
    priv var sectors: @{UInt16: TMPSectorOwners}

    init() {
        self.sectors <- {}
    }

    access(account) fun setWalletSize(sectorId: UInt16, address: Address, size: UInt16) {
      if self.sectors[sectorId] == nil {
          self.sectors[sectorId] <-! create TMPSectorOwners()
      }
      let sectorRef: &TMPSectorOwners = &self.sectors[sectorId] as &TMPSectorOwners
      sectorRef.setWalletSize(address: address, size: size)
    }

    access(account) fun setSaleSize(sectorId: UInt16, address: Address, size: UInt16) {
      let sectorRef: &TMPSectorOwners = &self.sectors[sectorId] as &TMPSectorOwners
      sectorRef.setSaleSize(address: address, size: size)
    }

    pub fun getOwners(sectorId: UInt16): {Address: TMPOwner} {
      if (self.sectors[sectorId] == nil) {
        return {}
      } else {
        let sectorRef: &TMPSectorOwners = &self.sectors[sectorId] as &TMPSectorOwners
        return sectorRef.getOwners()
      }
    }

    pub fun getOwner(sectorId: UInt16, address: Address): TMPOwner? {
      let sectorRef: &TMPSectorOwners = &self.sectors[sectorId] as &TMPSectorOwners
      return sectorRef.getOwner(address: address)
    }

    pub fun listSectors(): [UInt16] {
      return self.sectors.keys
    }

    destroy() {
        destroy self.sectors
    }
  }
}
