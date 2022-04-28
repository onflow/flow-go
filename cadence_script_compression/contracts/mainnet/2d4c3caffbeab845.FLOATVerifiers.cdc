// MADE BY: Emerald City, Jacob Tucker

// This contract is probably the most confusing element of the FLOAT
// platform. Listed here is a bunch of Structs which all implement
// FLOAT.IVerifier. 

// This pattern allows us to define arbitrary "restrictions" or "verifiers"
// on our FLOAT Events. For example, Timelock is a verifier that makes sure
// the current time is within the start and end date that the FLOAT Event host
// specified when they created an event.  

// The cool part is all of these verifiers are totally optional, and are only
// passed in with the newly created event if the host wanted to enable them.  
// You can mix and match them however you want. For example, one event I could
// use both Timelock and Limited, and for another event I could just use Secret.  

// Each verifier must have a `verify` function that takes in a generalized `params`
// argument so we can pass user data through as well as info about the event itself.  
// This is important for Secret for example because we want to pass the users guess
// of the secret code through. For Limited, we need to know the totalSupply of the event,
// so we pass it through as well.

import FLOAT from 0x2d4c3caffbeab845

pub contract FLOATVerifiers {

    // The "verifiers" to be used
    
    //
    // Timelock
    //
    // Specifies a time range in which the 
    // FLOAT from an event can be claimed
    pub struct Timelock: FLOAT.IVerifier {
        // An automatic switch handled by the contract
        // to stop people from claiming after a certain time.
        pub let dateStart: UFix64
        pub let dateEnding: UFix64

        pub fun verify(_ params: {String: AnyStruct}) {
            assert(
                getCurrentBlock().timestamp >= self.dateStart,
                message: "This FLOAT Event has not started yet."
            )
            assert(
                getCurrentBlock().timestamp <= self.dateEnding,
                message: "Sorry! The time has run out to mint this FLOAT."
            )
        }

        init(_dateStart: UFix64, _timePeriod: UFix64) {
            self.dateStart = _dateStart
            self.dateEnding = self.dateStart + _timePeriod
        }
    }

    //
    // Secret
    //
    // Specifies a secret code in order
    // to claim a FLOAT (not very secure, but cool feature)
    pub struct Secret: FLOAT.IVerifier {
        // The secret code, set by the owner of this event.
        access(self) let secretPhrase: String

        pub fun verify(_ params: {String: AnyStruct}) {
            let secretPhrase = params["secretPhrase"]! as! String
            assert(
                self.secretPhrase == secretPhrase, 
                message: "You did not input the correct secret phrase."
            )
        }

        init(_secretPhrase: String) {
            self.secretPhrase = _secretPhrase
        }
    }

    //
    // Limited
    //
    // Specifies a limit for the amount of people
    // who can CLAIM. Not to be confused with how many currently
    // hold a FLOAT from this event, since users can
    // delete their FLOATs.
    pub struct Limited: FLOAT.IVerifier {
        pub var capacity: UInt64

        pub fun verify(_ params: {String: AnyStruct}) {
            let event = params["event"]! as! &FLOAT.FLOATEvent{FLOAT.FLOATEventPublic}
            let currentCapacity = event.totalSupply
            assert(
                currentCapacity < self.capacity,
                message: "This FLOAT Event is at capacity."
            )
        }

        init(_capacity: UInt64) {
            self.capacity = _capacity
        }
    }

    //
    // MultipleSecret
    //
    // Allows for Multiple Secret codes
    // Everytime a secret gets used, it gets removed
    // from the list.
    pub struct MultipleSecret: FLOAT.IVerifier {
        access(self) let secrets: {String: Bool}

        pub fun verify(_ params: {String: AnyStruct}) {
            let secretPhrase = params["secretPhrase"]! as! String
            assert(
                self.secrets[secretPhrase] != nil, 
                message: "You did not input a correct secret phrase."
            )
            self.secrets.remove(key: secretPhrase)
        }

        init(_secrets: [String]) {
            self.secrets = {}
            for secret in _secrets {
                self.secrets[secret] = true
            }
        }
    }
}