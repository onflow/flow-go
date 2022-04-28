/**
SPDX-FileCopyrightText: 2021 copyright 52a74d3b580cdb48eb87979860ca6efe <creator@themasterpiece.art>
SPDX-License-Identifier: GPL-3.0-or-later

## `TheMasterPixelContract` contract

The contract will manage a collection of TheMasterPixel and colors organized by sector
Allowing anyone to hold TheMasterPixel NFTs


## `TheMasterSectors`, `TheMasterSector`, `TheMasterPixel`, `TheMasterPixelMinter` resources

TheMasterSectors will manage a dictionnary of TheMasterSector oraganized by sector
TheMasterSector will manage a dictionnary of TheMasterPixel organized by id and a dictionnary of colors
TheMasterPixel represents one pixel uniquely identified by its id on one sector
TheMasterPixelMinter will mint a set of TheMasterPixel for one sector

TheMasterSectors
    {sector -> TheMasterSector}
        {id ->  TheMasterPixel}
        {id ->  colors}

## `TheMasterSectorsInterface` resource interfaces

Exposes functions to query TheMasterPixel colors and ids owned by sector

*/

import TheMasterPieceContract from 0xcecd8c81c050eef1

pub contract TheMasterPixelContract {

  // Named Paths
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let CollectionPrivatePath: PrivatePath
  pub let MinterStoragePath: StoragePath


  init() {
      // Set our named paths
      self.CollectionStoragePath = /storage/TheMasterSectors
      self.CollectionPublicPath = /public/TheMasterSectors
      self.CollectionPrivatePath = /private/TheMasterSectors
      self.MinterStoragePath = /storage/TheMasterPixelMinter

      if (self.account.borrow<&TheMasterPixelMinter>(from: self.MinterStoragePath) == nil && self.account.borrow<&TheMasterSectors>(from: self.CollectionStoragePath) == nil) {
        self.account.save(<-create TheMasterPixelMinter(), to: self.MinterStoragePath)

        self.account.save(<-self.createEmptySectors(), to: self.CollectionStoragePath)
        self.account.link<&{TheMasterSectorsInterface}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
        self.account.link<&TheMasterSectors>(self.CollectionPrivatePath, target: self.CollectionStoragePath)
      }
  }

  // ########################################################################################

  pub resource TheMasterPixel {
    pub let id: UInt32

    init(id: UInt32) {
      self.id = id
    }

    destroy() {
    }
  }


  pub resource TheMasterPixelMinter {
      init() {
      }

      pub fun mintTheMasterPixel(theMasterSectorsRef: &TheMasterPixelContract.TheMasterSectors, ids: [UInt32], sector: UInt16, color: UInt32) {
        var idx = 0;
        while idx < 250 && idx < ids.length {
          theMasterSectorsRef.deposit(sectorId: sector,token: <- create TheMasterPixel(id: ids[idx]), color: color)
          idx = idx + 1;
        }
      }
  }


  // ########################################################################################

  pub resource TheMasterSector {
    priv var ownedNFTs: @{UInt32: TheMasterPixel}
    priv var colors: {UInt32: UInt32}
    pub var id: UInt16

    init (sectorId: UInt16) {
        self.id = sectorId
        self.ownedNFTs <- {}
        self.colors = {}
    }

    pub fun withdraw(withdrawID: UInt32): @TheMasterPixel {
        let token <- self.ownedNFTs.remove(key: withdrawID)!
        TheMasterPieceContract.setWalletSize(sectorId: self.id, address: (self.owner!).address, size: UInt16(self.ownedNFTs.length))
        return <-token
    }

    pub fun deposit(token: @TheMasterPixel, color: UInt32) {
        self.colors[token.id] = color
        self.ownedNFTs[token.id] <-! token
        TheMasterPieceContract.setWalletSize(sectorId: self.id, address: (self.owner!).address, size: UInt16(self.ownedNFTs.length))
    }

    pub fun setColor(id: UInt32, color: UInt32) {
        self.colors[id] = color
    }

    pub fun getColor(id: UInt32): UInt32 {
        return self.colors[id]!
    }

    pub fun removeColor(id: UInt32): UInt32 {
        if (!self.ownedNFTs.containsKey(id) && self.colors.containsKey(id)) {
          return self.colors.remove(key: id)!
        } else {
          return 0
        }
    }

    pub fun getPixels() : {UInt32: UInt32} {
        return self.colors
    }

    pub fun getIds() : [UInt32] {
        return self.ownedNFTs.keys
    }

    destroy() {
        destroy self.ownedNFTs
    }
  }

  // ########################################################################################

  pub resource interface TheMasterSectorsInterface {
    pub fun deposit(sectorId: UInt16, token: @TheMasterPixel, color: UInt32)
    pub fun getPixels(sectorId: UInt16) : {UInt32: UInt32}
    pub fun getIds(sectorId: UInt16) : [UInt32]
    pub fun getColor(sectorId: UInt16, id: UInt32): UInt32
    access(account) fun removeColor(sectorId: UInt16, id: UInt32): UInt32
  }

  pub fun createEmptySectors(): @TheMasterSectors {
      return <- create TheMasterSectors()
  }

  pub resource TheMasterSectors: TheMasterSectorsInterface {
    priv var ownedSectors: @{UInt16: TheMasterSector}

    init () {
        self.ownedSectors <- {}
    }

    pub fun deposit(sectorId: UInt16, token: @TheMasterPixel, color: UInt32) {
        if self.ownedSectors[sectorId] == nil {
            self.ownedSectors[sectorId] <-! create TheMasterSector(sectorId: sectorId)
        }

        let masterSectorRef: &TheMasterSector = &self.ownedSectors[sectorId] as &TheMasterSector
        masterSectorRef.deposit(token: <-token, color: color)
    }

    pub fun withdraw(sectorId: UInt16, withdrawID: UInt32): @TheMasterPixel {
        let masterSectorRef: &TheMasterSector = &self.ownedSectors[sectorId] as &TheMasterSector
        let token <- masterSectorRef.withdraw(withdrawID: withdrawID)!
        return <-token
    }

    pub fun getPixels(sectorId: UInt16) : {UInt32: UInt32} {
        if (self.ownedSectors.containsKey(sectorId)) {
          let masterSectorRef: &TheMasterSector = &self.ownedSectors[sectorId] as &TheMasterSector
          return masterSectorRef.getPixels()
        } else {
          return {}
        }
    }

    pub fun getIds(sectorId: UInt16) : [UInt32] {
        if (self.ownedSectors.containsKey(sectorId)) {
          let masterSectorRef: &TheMasterSector = &self.ownedSectors[sectorId] as &TheMasterSector
          return masterSectorRef.getIds()
        } else {
          return []
        }
    }

    pub fun setColor(sectorId: UInt16, id: UInt32, color: UInt32) {
        let masterSectorRef: &TheMasterSector = &self.ownedSectors[sectorId] as &TheMasterSector
        masterSectorRef.setColor(id: id, color: color)
    }

    pub fun getColor(sectorId: UInt16, id: UInt32): UInt32 {
        let masterSectorRef: &TheMasterSector = &self.ownedSectors[sectorId] as &TheMasterSector
        return masterSectorRef.getColor(id: id)
    }

    pub fun removeColor(sectorId: UInt16, id: UInt32): UInt32 {
        let masterSectorRef: &TheMasterSector = &self.ownedSectors[sectorId] as &TheMasterSector
        return masterSectorRef.removeColor(id: id)
    }

    destroy() {
        destroy self.ownedSectors
    }
  }

}
