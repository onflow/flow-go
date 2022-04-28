import KlktnNFT2 from 0xabd6e80be7e9682c
import KlktnNFTTimestamps from 0xabd6e80be7e9682c
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61

pub contract KlktnNFTProxy {

  // Emitted when KlktnNFT contract is created
  pub event ContractInitialized()

  // mintNFTWithFlow deposits FLOW & mints the next available serialNumber for NFT of typeID
  pub fun mintNFTWithFlow(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, paymentVault: @FungibleToken.Vault) {
    pre {
      !KlktnNFT2.isNFTTemplateExpired(typeID: typeID):
        "NFT of this typeID does not exist or is no longer being offered."
      paymentVault.balance >= KlktnNFT2.getNFTTemplateInfo(typeID: typeID).priceFlow:
        "Insufficient payment."
      paymentVault.getType() == Type<@FlowToken.Vault>():
        "payment type not FlowToken.Vault."
      KlktnNFTTimestamps.getNFTTemplateTimestamps(typeID: typeID).getTimestamps()["availableAt"]! <= getCurrentBlock().timestamp:
        "sale has not started"
      KlktnNFTTimestamps.getNFTTemplateTimestamps(typeID: typeID).getTimestamps()["expiresAt"]! >= getCurrentBlock().timestamp:
        "sale has ended"
    }
    let adminFlowReceiverRef = self.account.getCapability(/public/flowTokenReceiver).borrow<&{FungibleToken.Receiver}>()
      ?? panic("Could not borrow receiver reference to the admin's Vault")
    adminFlowReceiverRef.deposit(from: <-paymentVault)
    let admin = self.account.borrow<&KlktnNFT2.Admin>(from: KlktnNFT2.AdminStoragePath)
      ?? panic("Could not borrow a reference to the KlktnNFT2 Admin")
    admin.mintNextAvailableNFT(recipient: recipient, typeID: typeID, metadata: {})
  }

  init() {
    emit ContractInitialized()
  }
}
