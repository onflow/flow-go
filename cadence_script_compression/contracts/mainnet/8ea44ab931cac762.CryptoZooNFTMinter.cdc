// SPDX-License-Identifier: UNLICENSED

import CryptoZooNFT from 0x8ea44ab931cac762
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61

pub contract CryptoZooNFTMinter {

  pub event ContractInitialized()

  pub fun mintNFTWithFlow(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, paymentVault: @FungibleToken.Vault) {
    pre {
      !CryptoZooNFT.isNFTTemplateExpired(typeID: typeID):
        "invalid typeID"
      !CryptoZooNFT.getNFTTemplateByTypeID(typeID: typeID).isLand:
        "land cannot be purchased directly"  
      paymentVault.balance >= CryptoZooNFT.getNFTTemplateByTypeID(typeID: typeID).priceFlow:
        "Insufficient balance"
      paymentVault.getType() == Type<@FlowToken.Vault>():
        "invalid payment vault: not FlowToken"
      CryptoZooNFT.getNFTTemplateByTypeID(typeID: typeID).getTimestamps()["availableAt"]! <= getCurrentBlock().timestamp:
        "sale has not yet started"
      CryptoZooNFT.getNFTTemplateByTypeID(typeID: typeID).getTimestamps()["expiresAt"]! >= getCurrentBlock().timestamp:
        "sale has ended"
    }
    let adminFlowReceiverRef = self.account.getCapability(/public/flowTokenReceiver).borrow<&{FungibleToken.Receiver}>()
      ?? panic("Could not borrow receiver reference to the admin's Vault")
    adminFlowReceiverRef.deposit(from: <-paymentVault)
    let admin = self.account.borrow<&CryptoZooNFT.Admin>(from: CryptoZooNFT.AdminStoragePath)
      ?? panic("Could not borrow a reference to the CryptoZooNFT Admin")
    admin.mintNFT(recipient: recipient, typeID: typeID)
  }

  init() {
    emit ContractInitialized()
  }
}