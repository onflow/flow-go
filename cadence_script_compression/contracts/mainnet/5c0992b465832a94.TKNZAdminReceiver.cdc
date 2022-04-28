// Deployed for TKNZ Ltd. - https://tknz.gg

/*

  AdminReceiver.cdc

  This contract defines a function that takes a TKNZ Admin
  object and stores it in the storage of the contract account
  so it can be used.

 */

import TKNZ from 0x5c0992b465832a94

pub contract TKNZAdminReceiver {

    // storeAdmin takes a TKNZ Admin resource and 
    // saves it to the account storage of the account
    // where the contract is deployed
    pub fun storeAdmin(newAdmin: @TKNZ.Admin) {
        self.account.save(<-newAdmin, to: /storage/TKNZAdmin)
    }
    
    init() {

    }
}
