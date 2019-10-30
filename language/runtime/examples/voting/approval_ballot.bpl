
/*
*   WARNING: Don't completely trust that this code is correct.  It is just practice
*
*   In this example, we want to create a simple approval voting contract where a polling place 
*   issues ballots to addresses. 
*   
*   When the polling place is created, it is initialized with an array of proposals and 
*   every user with a ballot is allowed to approve any number of proposals.
*
*   TODO: Add functionality to send a ballot to someone else so they can vote for you.
*         Might use an NFT for this
* 
*/

import PollingPlace from "ballot.bpl"

// This contract implements the voting functionality using the PollingPlace interface
pub contract MultiApprovalBallot: PollingPlace {

    // This is the resource that is issued to users, they modify to include their vote,
    // and then resubmit to have their vote included in the polling
    pub resource Ballot {

        // array of all the proposals 
        pub let proposals: [String]
        // corresponds to an array index in proposals after a vote
        pub var choices: {Int: Bool}

        // creator must supply the array of proposals on init
        init(proposals: [String]) {
            self.proposals = proposals
            self.choices = {}
        }

        // modifies the ballot in storage to indicate which proposals it is voting for
        pub fun vote(proposal: Int) {
            pre {
                proposal <= self.proposals.length
            }
            self.choices[proposal] = true
        }
    }

    pub let ballotIssued: {Address: bool}  // shows that an account has been issued a ballot
    pub let proposals: [String]            // list of proposals to be approved
    pub let votes: {Int: Int}           // number of votes per proposal

    // initializes the proposals array so we can see what is being voted on
    init(proposals: [String]) {
        self.proposals = proposals
        self.votes = {}
    }

    // A user moves their ballot to this function where its votes are tallied and the ballot is destroyed
    pub fun cast(ballot: <-Ballot) {
        var index = 0
        while (index < proposals.length) {
            if (ballot.choices[index]) {
                self.votes[index] = self.votes[index] + 1
            }
            index = index + 1;
        }

        destroy ballot
    }

    // Any account can call this function to be issued a ballot
    // can only be called once per account
    pub fun issueBallot(): <-Ballot {
        pre {
            !self.ballotIssued[MSG.SENDER]  //TODO: Need to figure out how to get the caller's address
                "caller has already been issued a ballot!"
        }
        self.ballotIssued[MSG.SENDER] == true
        return <-create Ballot(proposals: self.proposals)
    }

    // Combines the issuing, voting, and casting a ballot all into one function call
    pub fun issueAndCast(proposals: [Int]) {
        let ballot: <- self.issueBallot()

        var index = 0
        // record vote for each proposal submitted
        while (index < proposals.length) {
            ballot.vote(proposals[index])
            index = index + 1
        }
        
        // cast the whole ballot
        self.cast(<-ballot)
    }
}
