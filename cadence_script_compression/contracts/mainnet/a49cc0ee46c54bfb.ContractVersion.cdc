//Interface to enforce version on a contract
//This interface will be used by migration tools to manage versions for updates.
//If an existing 'old' contract can't implement this interface by updating,
//then add a getVersion method to that contract without the interface
//
pub contract interface ContractVersion {
    pub fun getVersion():String
}