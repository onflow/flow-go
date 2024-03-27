import NodeVersionBeacon from 0xNODEVERSIONBEACONADDRESS

/// Transaction that allows NodeVersionAdmin to specify a new protocol state version.
/// The new version will become active at view `activeView` if the service event
/// is processed and applied to the protocol state within a block `B` such that
/// `B.view + ∆ < activeView`, for a protocol-defined safety threshold ∆.
/// Service events not meeting this threshold are discarded.
///
/// This is a special version of the admin transaction for use in integration tests.
/// We allow the sender to pass in a value to add to the current view, to reduce
/// the liklihood that a test spuriously fails due to timing.

transaction(newProtocolVersion: UInt64, activeViewDiff: UInt64) {

  let adminRef: &NodeVersionBeacon.Admin

  prepare(acct: AuthAccount) {
    // Borrow a reference to the NodeVersionAdmin implementing resource
    self.adminRef = acct.borrow<&NodeVersionBeacon.Admin>(from: NodeVersionBeacon.AdminStoragePath)
      ?? panic("Couldn't borrow NodeVersionBeacon.Admin Resource")
  }

  execute {
    let block = getCurrentBlock()
    self.adminRef.setPendingProtocolStateVersionUpgrade(newProtocolVersion: newProtocolVersion, activeView: block.view + activeViewDiff)
  }
}
